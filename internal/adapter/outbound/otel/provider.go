package otel

import (
	"context"
	"net/http"
	"strings"
	"time"

	promcli "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider wraps OTEL tracer/meter providers and metrics helpers
type Provider struct {
	ServiceName string
	Version     string
	Environment string

	TraceProvider  *sdktrace.TracerProvider
	MetricProvider *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter

	logCounter   metric.Int64Counter
	errorCounter metric.Int64Counter
	durationHist metric.Float64Histogram

	propagator   propagation.TextMapPropagator
	promRegistry *promcli.Registry
}

func NewOTelProvider(serviceName, version, environment, endpoint string) (*Provider, error) {
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	endpoint = sanitizeGrpcEndpoint(endpoint)
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes("", semconv.ServiceName(serviceName), semconv.ServiceVersion(version), semconv.DeploymentEnvironment(environment)))
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.AlwaysSample()))
	promReg := promcli.NewRegistry()
	promExporter, err := prometheus.New(prometheus.WithRegisterer(promReg))
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExporter), sdkmetric.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(propagator)
	p := &Provider{ServiceName: serviceName, Version: version, Environment: environment, TraceProvider: tp, MetricProvider: mp, tracer: tp.Tracer(serviceName), meter: mp.Meter(serviceName), propagator: propagator, promRegistry: promReg}
	if err := p.initMetrics(); err != nil {
		return nil, err
	}
	return p, nil
}

func sanitizeGrpcEndpoint(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return endpoint
}

func (p *Provider) initMetrics() error {
	var err error
	p.logCounter, err = p.meter.Int64Counter("logs_total", metric.WithDescription("Total number of logs written"))
	if err != nil {
		return err
	}
	p.errorCounter, err = p.meter.Int64Counter("errors_total", metric.WithDescription("Total number of error logs"))
	if err != nil {
		return err
	}
	p.durationHist, err = p.meter.Float64Histogram("log_duration_seconds", metric.WithDescription("Time spent writing logs"), metric.WithUnit("s"))
	return err
}

func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}
func (p *Provider) ExtractTraceInfo(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return
}
func (p *Provider) RecordLogMetric(ctx context.Context, d time.Duration, level string, attrs ...attribute.KeyValue) {
	if ctx == nil {
		ctx = context.Background()
	}
	p.logCounter.Add(ctx, 1, metric.WithAttributes(append(attrs, attribute.String("level", level))...))
	if level == "ERROR" || level == "FATAL" {
		p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	p.durationHist.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}
func (p *Provider) InjectHeaders(ctx context.Context, headers http.Header) {
	p.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}
func (p *Provider) ExtractHeaders(ctx context.Context, headers http.Header) context.Context {
	return p.propagator.Extract(ctx, propagation.HeaderCarrier(headers))
}
func (p *Provider) PrometheusHandler() http.Handler {
	if p.promRegistry == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(p.promRegistry, promhttp.HandlerOpts{})
}
func (p *Provider) Shutdown(ctx context.Context) error {
	if err := p.TraceProvider.Shutdown(ctx); err != nil {
		return err
	}
	return p.MetricProvider.Shutdown(ctx)
}

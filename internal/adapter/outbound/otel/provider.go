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
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
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

// NewOTelProvider builds the tracer + meter provider.
//
// The OTLP exporters are gated on endpoint: with a non-empty endpoint,
// traces and metrics are exported to the collector. With an EMPTY
// endpoint the providers are still built (AlwaysSample tracer, in-process
// prometheus meter) but nothing is exported — this is the "no OTLP
// exporter" mode. The distinction matters because boeng always wants a
// real tracer so spans get valid W3C contexts (trace_id in logs, and
// traceparent propagation across process boundaries) even when no
// collector is configured; only the export is optional.
func NewOTelProvider(serviceName, version, environment, endpoint string) (*Provider, error) {
	exporting := endpoint != ""
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes("", semconv.ServiceName(serviceName), semconv.ServiceVersion(version), semconv.DeploymentEnvironment(environment)))
	if err != nil {
		return nil, err
	}

	// When exporting, record fully (AlwaysSample). When not exporting,
	// use NeverSample: the tracer still generates valid W3C span contexts
	// — so trace_id/span_id reach the logs and traceparent propagates —
	// but the spans are non-recording, avoiding the cost of recording
	// attributes nothing will ever export.
	sampler := sdktrace.NeverSample()
	if exporting {
		sampler = sdktrace.AlwaysSample()
	}
	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res), sdktrace.WithSampler(sampler)}
	promReg := promcli.NewRegistry()
	promExporter, err := prometheus.New(prometheus.WithRegisterer(promReg))
	if err != nil {
		return nil, err
	}
	metricOpts := []sdkmetric.Option{sdkmetric.WithReader(promExporter), sdkmetric.WithResource(res)}

	if exporting {
		ep := sanitizeGrpcEndpoint(endpoint)
		traceExporter, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithEndpoint(ep), otlptracegrpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(traceExporter))
		// Metrics leave the process via OTLP the same way traces do; the
		// prometheus reader above only feeds the in-process registry.
		metricExporter, err := otlpmetricgrpc.New(context.Background(), otlpmetricgrpc.WithEndpoint(ep), otlpmetricgrpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		metricOpts = append(metricOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second))))
	}

	tp := sdktrace.NewTracerProvider(traceOpts...)
	mp := sdkmetric.NewMeterProvider(metricOpts...)

	// Always install the tracer + propagator so span contexts and W3C
	// propagation work regardless of export. Only take over the global
	// meter provider when actually exporting — otherwise leave metric
	// recording as a no-op (boeng's per-op metrics read the global meter)
	// so a no-collector deployment keeps its previous zero-metric cost.
	otel.SetTracerProvider(tp)
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(propagator)
	if exporting {
		otel.SetMeterProvider(mp)
	}

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

package shared

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	pin "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
)

// OTelProvider provides OpenTelemetry integration
type OTelProvider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter
	propagator     propagation.TextMapPropagator

	// Metrics
	logCounter     metric.Int64Counter
	logLatency     metric.Float64Histogram
	errorCounter   metric.Int64Counter
	activeRequests metric.Int64UpDownCounter
}

// NewOTelProvider creates new OpenTelemetry provider
func NewOTelProvider(serviceName, endpoint string, insecure bool) (*OTelProvider, error) {
	// Check if there's already a global tracer provider
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok && tp != nil {
		// Use existing provider
		meter := otel.GetMeterProvider().Meter("logbrutal")
		return &OTelProvider{
			tracerProvider: tp,
			meterProvider:  nil, // Can't get meter provider from global
			tracer:         tp.Tracer("logbrutal"),
			meter:          meter,
			propagator:     otel.GetTextMapPropagator(),
		}, nil
	}

	// Create resource without schema URL to avoid conflicts
	res := resource.NewWithAttributes(
		"", // Empty schema URL to avoid conflicts
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion("1.0.0"),
		attribute.String("environment", "production"),
	)

	// Create trace exporter
	ctx := context.Background()

	var dialOpts []grpc.DialOption
	if insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	}

	// Try to connect to OTLP endpoint with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, dialOpts...)
	if err != nil {
		// Log warning but continue without tracing
		fmt.Printf("Warning: Failed to connect to OTLP endpoint %s: %v. Tracing disabled.\n", endpoint, err)
		return &OTelProvider{
			tracerProvider: nil,
			meterProvider:  nil,
			tracer:         nil,
			meter:          nil,
			propagator:     otel.GetTextMapPropagator(),
		}, nil
	}

	traceExporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
		otlptracegrpc.WithGRPCConn(conn),
	))
	if err != nil {
		// Log warning but continue without tracing
		fmt.Printf("Warning: Failed to create trace exporter: %v. Tracing disabled.\n", err)
		conn.Close()
		return &OTelProvider{
			tracerProvider: nil,
			meterProvider:  nil,
			tracer:         nil,
			meter:          nil,
			propagator:     otel.GetTextMapPropagator(),
		}, nil
	}

	// Create tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Create Prometheus exporter for metrics
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	// Create meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)

	// Set globals
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create meters
	meter := meterProvider.Meter("logbrutal")

	logCounter, err := meter.Int64Counter(
		"logbrutal.logs.total",
		metric.WithDescription("Total number of logs"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	logLatency, err := meter.Float64Histogram(
		"logbrutal.log.latency",
		metric.WithDescription("Log processing latency"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	errorCounter, err := meter.Int64Counter(
		"logbrutal.errors.total",
		metric.WithDescription("Total number of errors logged"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"logbrutal.active_requests",
		metric.WithDescription("Number of active requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	return &OTelProvider{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		tracer:         tracerProvider.Tracer("logbrutal"),
		meter:          meter,
		propagator:     otel.GetTextMapPropagator(),
		logCounter:     logCounter,
		logLatency:     logLatency,
		errorCounter:   errorCounter,
		activeRequests: activeRequests,
	}, nil
}

// Shutdown gracefully shuts down the provider
func (p *OTelProvider) Shutdown(ctx context.Context) error {
	if err := p.tracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	return p.meterProvider.Shutdown(ctx)
}

// ExtractContext extracts trace context from HTTP headers
func (p *OTelProvider) ExtractContext(r *http.Request) context.Context {
	return p.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
}

// InjectContext injects trace context into HTTP headers
func (p *OTelProvider) InjectContext(ctx context.Context, r *http.Request) {
	p.propagator.Inject(ctx, propagation.HeaderCarrier(r.Header))
}

// RecordLog records log metrics
func (p *OTelProvider) RecordLog(ctx context.Context, level domain.Level, module string, latency time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("level", level.String()),
		attribute.String("module", module),
	}

	p.logCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	p.logLatency.Record(ctx, latency.Seconds()*1000, metric.WithAttributes(attrs...))

	if level >= domain.ErrorLevel {
		p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// IncrementActiveRequests increments active request counter
func (p *OTelProvider) IncrementActiveRequests(ctx context.Context) {
	p.activeRequests.Add(ctx, 1)
}

// DecrementActiveRequests decrements active request counter
func (p *OTelProvider) DecrementActiveRequests(ctx context.Context) {
	p.activeRequests.Add(ctx, -1)
}

// Helper functions for context

func extractTraceIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return ""
}

func extractSpanIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.SpanID().String()
	}
	return ""
}

// InjectAMQPHeaders injects trace context into AMQP headers
func InjectAMQPHeaders(ctx context.Context, headers map[string]interface{}) {
	propagator := otel.GetTextMapPropagator()
	carrier := &amqpHeaderCarrier{headers: headers}
	propagator.Inject(ctx, carrier)
}

// ExtractAMQPHeaders extracts trace context from AMQP headers
func ExtractAMQPHeaders(ctx context.Context, headers map[string]interface{}) context.Context {
	propagator := otel.GetTextMapPropagator()
	carrier := &amqpHeaderCarrier{headers: headers}
	return propagator.Extract(ctx, carrier)
}

// amqpHeaderCarrier implements TextMapCarrier for AMQP headers
type amqpHeaderCarrier struct {
	headers map[string]interface{}
}

func (c *amqpHeaderCarrier) Get(key string) string {
	if val, ok := c.headers[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (c *amqpHeaderCarrier) Set(key, value string) {
	c.headers[key] = value
}

func (c *amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// StartSpan starts a new span with common attributes
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Check if tracing is available
	if otel.GetTracerProvider() == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := otel.Tracer("logbrutal")
	if tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	return tracer.Start(ctx, name, opts...)
}

// StartSpanWithLogger starts a span and returns logger with trace context
func StartSpanWithLogger(ctx context.Context, logger pin.Logger, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span, pin.Logger) {
	ctx, span := StartSpan(ctx, name, opts...)
	newLogger := logger.Ctx(ctx)
	return ctx, span, newLogger
}

// RecordError records error in span
func RecordError(span trace.Span, err error, msg string) {
	if err == nil {
		return
	}

	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.message", msg),
	))
	span.SetStatus(codes.Error, err.Error())
}

// PrometheusHandler returns HTTP handler for Prometheus metrics
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

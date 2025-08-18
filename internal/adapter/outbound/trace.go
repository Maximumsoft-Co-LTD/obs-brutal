package outbound

import (
	"context"
	"fmt"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/outbound"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OTelTraceProvider provides distributed tracing using OpenTelemetry
type OTelTraceProvider struct {
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	propagator     propagation.TextMapPropagator
}

// NewOTelTraceProvider creates a new OpenTelemetry trace provider
func NewOTelTraceProvider(serviceName, endpoint string, insecureConn bool) (outbound.TraceSrc, error) {
	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP exporter
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var opts []otlptracegrpc.Option
	if insecureConn {
		conn, err := grpc.DialContext(ctx, endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
		}
		opts = append(opts, otlptracegrpc.WithGRPCConn(conn))
	} else {
		opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
	}

	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	return &OTelTraceProvider{
		tracerProvider: tracerProvider,
		tracer:         tracerProvider.Tracer(serviceName),
		propagator:     propagator,
	}, nil
}

func (p *OTelTraceProvider) TraceID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return ""
}

func (p *OTelTraceProvider) SpanID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.SpanID().String()
	}
	return ""
}

func (p *OTelTraceProvider) StartSpan(ctx context.Context, name string) (context.Context, outbound.Span) {
	ctx, span := p.tracer.Start(ctx, name)
	return ctx, &otelSpan{span: span}
}

func (p *OTelTraceProvider) Tracer(name string) outbound.Tracer {
	return &otelTracer{
		tracer: p.tracerProvider.Tracer(name),
	}
}

func (p *OTelTraceProvider) Health() error {
	// Try to create a test span
	_, span := p.tracer.Start(context.Background(), "health-check")
	defer span.End()

	// If we can create a span, we're healthy
	return nil
}

func (p *OTelTraceProvider) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.tracerProvider.Shutdown(ctx)
}

// otelSpan implements Span interface
type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) Attr(key string, value interface{}) {
	attr := attributeFromValue(key, value)
	if attr.Key != "" {
		s.span.SetAttributes(attr)
	}
}

func (s *otelSpan) Attrs(attrs map[string]interface{}) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		attr := attributeFromValue(k, v)
		if attr.Key != "" {
			otelAttrs = append(otelAttrs, attr)
		}
	}
	s.span.SetAttributes(otelAttrs...)
}

func (s *otelSpan) Err(err error) {
	if err != nil {
		s.span.RecordError(err)
	}
}

func (s *otelSpan) Status(code outbound.SpanStatusCode, description string) {
	var otelCode codes.Code
	switch code {
	case outbound.SpanStatusOK:
		otelCode = codes.Ok
	case outbound.SpanStatusError:
		otelCode = codes.Error
	default:
		otelCode = codes.Unset
	}
	s.span.SetStatus(otelCode, description)
}

func (s *otelSpan) End() {
	s.span.End()
}

// otelTracer implements Tracer interface
type otelTracer struct {
	tracer trace.Tracer
}

func (t *otelTracer) Start(ctx context.Context, spanName string) (context.Context, outbound.Span) {
	ctx, span := t.tracer.Start(ctx, spanName)
	return ctx, &otelSpan{span: span}
}

// NoOpTraceProvider provides a no-op trace provider
type NoOpTraceProvider struct{}

// NewNoOpTraceProvider creates a no-op trace provider
func NewNoOpTraceProvider() outbound.TraceSrc {
	return &NoOpTraceProvider{}
}

func (p *NoOpTraceProvider) TraceID(ctx context.Context) string {
	return ""
}

func (p *NoOpTraceProvider) SpanID(ctx context.Context) string {
	return ""
}

func (p *NoOpTraceProvider) StartSpan(ctx context.Context, name string) (context.Context, outbound.Span) {
	return ctx, &noOpSpan{}
}

func (p *NoOpTraceProvider) Tracer(name string) outbound.Tracer {
	return &noOpTracer{}
}

func (p *NoOpTraceProvider) Health() error {
	return nil
}

func (p *NoOpTraceProvider) Close() error {
	return nil
}

// noOpSpan implements Span interface with no operations
type noOpSpan struct{}

func (s *noOpSpan) Attr(key string, value interface{})                      {}
func (s *noOpSpan) Attrs(attrs map[string]interface{})                      {}
func (s *noOpSpan) Err(err error)                                           {}
func (s *noOpSpan) Status(code outbound.SpanStatusCode, description string) {}
func (s *noOpSpan) End()                                                    {}

// noOpTracer implements Tracer interface with no operations
type noOpTracer struct{}

func (t *noOpTracer) Start(ctx context.Context, spanName string) (context.Context, outbound.Span) {
	return ctx, &noOpSpan{}
}

// Helper functions

func attributeFromValue(key string, value interface{}) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	default:
		// Convert to string
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

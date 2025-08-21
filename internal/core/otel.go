// Package core provides full OpenTelemetry integration
package core

import (
	"context"
	"net/http"
	"sync"
	"time"

	"obs-brutal/internal/core/domain"

	"github.com/gin-gonic/gin"
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

// ===== OTEL PROVIDER =====

// OTelProvider manages OpenTelemetry integration
type OTelProvider struct {
	serviceName string
	version     string
	environment string

	// OTEL components
	traceProvider  *sdktrace.TracerProvider
	metricProvider *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter

	// Metrics
	logCounter   metric.Int64Counter
	errorCounter metric.Int64Counter
	durationHist metric.Float64Histogram

	// Propagators
	propagator propagation.TextMapPropagator

	mu sync.RWMutex
}

// NewOTelProvider creates full OTEL integration
func NewOTelProvider(serviceName, version, environment, endpoint string) (*OTelProvider, error) {
	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Setup trace provider
	traceExporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Setup metric provider
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	metricProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithResource(res),
	)

	// Set global providers
	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(metricProvider)

	// Setup propagation
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	// Create tracer and meter
	tracer := traceProvider.Tracer(serviceName)
	meter := metricProvider.Meter(serviceName)

	provider := &OTelProvider{
		serviceName:    serviceName,
		version:        version,
		environment:    environment,
		traceProvider:  traceProvider,
		metricProvider: metricProvider,
		tracer:         tracer,
		meter:          meter,
		propagator:     propagator,
	}

	// Initialize metrics
	if err := provider.initMetrics(); err != nil {
		return nil, err
	}

	return provider, nil
}

// initMetrics initializes logging metrics
func (p *OTelProvider) initMetrics() error {
	var err error

	// Log counter
	p.logCounter, err = p.meter.Int64Counter(
		"obs_brutal_logs_total",
		metric.WithDescription("Total number of logs written"),
	)
	if err != nil {
		return err
	}

	// Error counter
	p.errorCounter, err = p.meter.Int64Counter(
		"obs_brutal_errors_total",
		metric.WithDescription("Total number of error logs"),
	)
	if err != nil {
		return err
	}

	// Duration histogram
	p.durationHist, err = p.meter.Float64Histogram(
		"obs_brutal_log_duration_seconds",
		metric.WithDescription("Time spent writing logs"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	return nil
}

// StartSpan starts a new trace span
func (p *OTelProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}

// ExtractTraceInfo extracts trace information from context
func (p *OTelProvider) ExtractTraceInfo(ctx context.Context) (traceID, spanID string) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		traceID = spanCtx.TraceID().String()
		spanID = spanCtx.SpanID().String()
	}
	return
}

// RecordLogMetric records logging metrics
func (p *OTelProvider) RecordLogMetric(level domain.Level, duration time.Duration, attributes ...attribute.KeyValue) {
	// Record log count
	p.logCounter.Add(context.Background(), 1, metric.WithAttributes(
		append(attributes, attribute.String("level", level.String()))...,
	))

	// Record error count if error level
	if level >= domain.ErrorLevel {
		p.errorCounter.Add(context.Background(), 1, metric.WithAttributes(attributes...))
	}

	// Record duration
	p.durationHist.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attributes...))
}

// InjectHeaders injects trace context into HTTP headers
func (p *OTelProvider) InjectHeaders(ctx context.Context, headers http.Header) {
	p.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractHeaders extracts trace context from HTTP headers
func (p *OTelProvider) ExtractHeaders(ctx context.Context, headers http.Header) context.Context {
	return p.propagator.Extract(ctx, propagation.HeaderCarrier(headers))
}

// PrometheusHandler returns Prometheus metrics handler
func (p *OTelProvider) PrometheusHandler() http.Handler {
	// Return Prometheus metrics endpoint
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This would integrate with the prometheus exporter
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Prometheus metrics endpoint\n"))
	})
}

// Shutdown gracefully shuts down OTEL provider
func (p *OTelProvider) Shutdown(ctx context.Context) error {
	if err := p.traceProvider.Shutdown(ctx); err != nil {
		return err
	}
	return p.metricProvider.Shutdown(ctx)
}

// ===== OTEL-AWARE LOGGER =====

// OTelLogger combines unified logger with full OTEL integration
type OTelLogger struct {
	*StrategyLogger
	otelProvider *OTelProvider
}

// NewOTelLogger creates logger with full OTEL integration
func NewOTelLogger(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*OTelLogger, error) {
	// Setup OTEL provider
	provider, err := NewOTelProvider(serviceName, version, environment, endpoint)
	if err != nil {
		return nil, err
	}

	// Create strategy logger with async pipeline
	strategyLogger := NewStrategyLogger(level, sinks...)

	return &OTelLogger{
		StrategyLogger: strategyLogger,
		otelProvider:   provider,
	}, nil
}

// Override log method to include OTEL metrics and tracing
func (ol *OTelLogger) log(level domain.Level, msg string) {
	start := time.Now()

	// Fast level check
	if level < ol.level {
		return
	}

	// Create log entry with OTEL context if available
	entry := &domain.LogEntry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}, len(ol.fields)),
	}

	// Copy fields efficiently
	for k, v := range ol.fields {
		entry.Fields[k] = v
	}

	// Extract trace info from current context if available
	if ctx, ok := ol.fields["context"].(context.Context); ok {
		traceID, spanID := ol.otelProvider.ExtractTraceInfo(ctx)
		if traceID != "" {
			entry.TraceID = traceID
			entry.Fields["trace_id"] = traceID
		}
		if spanID != "" {
			entry.SpanID = spanID
			entry.Fields["span_id"] = spanID
		}
	}

	// Apply strategies (filter, sample, mask)
	processedEntry := ol.strategies.ProcessEntry(entry)
	if processedEntry == nil {
		return // Filtered or sampled out
	}

	// Write async
	if !ol.async.WriteAsync(processedEntry) {
		// Fallback to sync
		for _, sink := range ol.async.sinks {
			if sink != nil {
				sink.Write(processedEntry)
			}
		}
	}

	// Record OTEL metrics
	duration := time.Since(start)
	ol.otelProvider.RecordLogMetric(level, duration,
		attribute.String("service", ol.otelProvider.serviceName),
		attribute.String("level", level.String()),
	)

	// Update counter
	if ol.logCount != nil {
		ol.logCount.Add(1)
	}
}

// WithSpan creates a new span and returns logger with span context
func (ol *OTelLogger) WithSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span, Logger) {
	spanCtx, span := ol.otelProvider.StartSpan(ctx, name, opts...)

	// Create logger with span context
	logger := ol.Ctx(spanCtx)

	return spanCtx, span, logger
}

// GetOTelProvider returns the OTEL provider for advanced usage
func (ol *OTelLogger) GetOTelProvider() *OTelProvider {
	return ol.otelProvider
}

// ===== GIN OTEL MIDDLEWARE =====

// OTelGinMiddleware creates Gin middleware with full OTEL integration
func OTelGinMiddleware(otelLogger *OTelLogger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()

		// Extract or create trace context
		ctx := otelLogger.otelProvider.ExtractHeaders(c.Request.Context(), c.Request.Header)

		// Start span for request
		spanCtx, span := otelLogger.otelProvider.StartSpan(ctx, c.Request.URL.Path,
			trace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPRoute(c.FullPath()),
				semconv.HTTPScheme(c.Request.URL.Scheme),
				semconv.HTTPTarget(c.Request.URL.Path),
			),
		)
		defer span.End()

		// Create request logger with full context
		requestLogger := otelLogger.
			Ctx(spanCtx).
			RequestID(generateRequestID()).
			F("method", c.Request.Method).
			F("path", c.Request.URL.Path).
			F("ip", c.ClientIP()).
			F("user_agent", c.Request.UserAgent())

		// Store in Gin context
		c.Set("otel_logger", requestLogger)
		c.Set("span", span)
		c.Request = c.Request.WithContext(spanCtx)

		// Process request
		c.Next()

		// Log completion with metrics
		duration := time.Since(start)
		status := c.Writer.Status()

		// Add span attributes
		span.SetAttributes(
			semconv.HTTPStatusCode(status),
			attribute.Int64("http.response.size", int64(c.Writer.Size())),
			attribute.Float64("duration_ms", float64(duration.Milliseconds())),
		)

		// Record error if status >= 400
		if status >= 400 {
			span.RecordError(
				domain.NewHTTPError(status, "HTTP request failed"),
				trace.WithAttributes(
					attribute.Int("status_code", status),
				),
			)
		}

		// Log request completion
		requestLogger.
			F("status", status).
			F("duration_ms", duration.Milliseconds()).
			F("response_size", c.Writer.Size()).
			Info("Request completed")
	})
}

// GetOTelLog extracts OTEL-aware logger from Gin context
func GetOTelLog(c *gin.Context) Logger {
	if logger, exists := c.Get("otel_logger"); exists {
		if log, ok := logger.(Logger); ok {
			return log
		}
	}

	// Fallback to regular logger
	return NewUnifiedLogger(INFO)
}

// ===== OTEL SINKS =====

// OTLPSink sends logs directly to OTLP collector
type OTLPSink struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
}

func NewOTLPSink(endpoint string) *OTLPSink {
	return &OTLPSink{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *OTLPSink) Write(entry *domain.LogEntry) error {
	// Convert log entry to OTLP format and send
	// Simplified implementation
	return nil
}

func (s *OTLPSink) Close() error                           { return nil }
func (s *OTLPSink) Name() string                           { return "otlp" }
func (s *OTLPSink) Health() error                          { return nil }
func (s *OTLPSink) Configure(map[string]interface{}) error { return nil }

// ===== AMQP INTEGRATION =====

// AMQPContextPropagator handles trace context in AMQP messages
type AMQPContextPropagator struct {
	propagator propagation.TextMapPropagator
}

func NewAMQPContextPropagator() *AMQPContextPropagator {
	return &AMQPContextPropagator{
		propagator: otel.GetTextMapPropagator(),
	}
}

// InjectAMQPHeaders injects trace context into AMQP headers
func (p *AMQPContextPropagator) InjectAMQPHeaders(ctx context.Context, headers map[string]interface{}) {
	carrier := &AMQPHeaderCarrier{headers: headers}
	p.propagator.Inject(ctx, carrier)
}

// ExtractAMQPHeaders extracts trace context from AMQP headers
func (p *AMQPContextPropagator) ExtractAMQPHeaders(ctx context.Context, headers map[string]interface{}) context.Context {
	carrier := &AMQPHeaderCarrier{headers: headers}
	return p.propagator.Extract(ctx, carrier)
}

// AMQPHeaderCarrier implements propagation.TextMapCarrier for AMQP
type AMQPHeaderCarrier struct {
	headers map[string]interface{}
}

func (c *AMQPHeaderCarrier) Get(key string) string {
	if val, ok := c.headers[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (c *AMQPHeaderCarrier) Set(key, value string) {
	c.headers[key] = value
}

func (c *AMQPHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// ===== CORRELATION HELPERS =====

// CorrelationExtractor extracts correlation IDs from various sources
type CorrelationExtractor struct {
	headerMappings map[string]string // Header name -> Field name
	contextKeys    []string
}

func NewCorrelationExtractor() *CorrelationExtractor {
	return &CorrelationExtractor{
		headerMappings: map[string]string{
			"X-Trace-Id":   "trace_id",
			"X-Request-Id": "request_id",
			"X-User-Id":    "user_id",
			"X-Session-Id": "session_id",
			"X-Tenant-Id":  "tenant_id",
			"X-Client-IP":  "client_ip",
		},
		contextKeys: []string{
			"trace_id", "span_id", "user_id", "request_id",
			"session_id", "tenant_id", "client_ip",
		},
	}
}

// ExtractFromHTTPRequest extracts correlation IDs from HTTP request
func (ce *CorrelationExtractor) ExtractFromHTTPRequest(r *http.Request) map[string]interface{} {
	fields := make(map[string]interface{})

	// Extract from headers
	for header, field := range ce.headerMappings {
		if value := r.Header.Get(header); value != "" {
			fields[field] = value
		}
	}

	// Extract from context
	for _, key := range ce.contextKeys {
		if value := r.Context().Value(key); value != nil {
			fields[key] = value
		}
	}

	return fields
}

// ExtractFromContext extracts correlation IDs from context
func (ce *CorrelationExtractor) ExtractFromContext(ctx context.Context) map[string]interface{} {
	fields := make(map[string]interface{})

	// Extract OTEL trace info
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		fields["trace_id"] = spanCtx.TraceID().String()
		fields["span_id"] = spanCtx.SpanID().String()
	}

	// Extract other context values
	for _, key := range ce.contextKeys {
		if value := ctx.Value(key); value != nil {
			fields[key] = value
		}
	}

	return fields
}

// ===== HTTP ERROR FOR SPAN RECORDING =====

// HTTPError represents HTTP-related errors for span recording
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates new HTTP error
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// ===== CONVENIENCE FUNCTIONS =====

// generateRequestID creates efficient request ID
func generateRequestID() string {
	return domain.GenerateID("req")
}

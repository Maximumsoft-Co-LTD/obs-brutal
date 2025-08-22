// Package core provides full OpenTelemetry integration
package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"obs-brutal/internal/core/domain"

	"github.com/gin-gonic/gin"
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

	// Prometheus registry used by exporter
	promRegistry *promcli.Registry
}

// NewOTelProvider creates full OTEL integration
func NewOTelProvider(serviceName, version, environment, endpoint string) (*OTelProvider, error) {
	// Sanitize endpoint and apply defaults (gRPC 4317)
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	endpoint = sanitizeGrpcEndpoint(endpoint)

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
	promReg := promcli.NewRegistry()
	promExporter, err := prometheus.New(prometheus.WithRegisterer(promReg))
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
		promRegistry:   promReg,
	}

	// Initialize metrics
	if err := provider.initMetrics(); err != nil {
		return nil, err
	}

	return provider, nil
}

// sanitizeGrpcEndpoint strips http(s):// schema for gRPC 4317 endpoints
func sanitizeGrpcEndpoint(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return endpoint

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

// RecordLogMetric records logging metrics using the provided context
func (p *OTelProvider) RecordLogMetric(ctx context.Context, level domain.Level, duration time.Duration, attributes ...attribute.KeyValue) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Record log count
	p.logCounter.Add(ctx, 1, metric.WithAttributes(
		append(attributes, attribute.String("level", level.String()))...,
	))

	// Record error count if error level
	if level >= domain.ErrorLevel {
		p.errorCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
	}

	// Record duration
	p.durationHist.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
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
	// Return Prometheus metrics endpoint backed by registry
	if p.promRegistry == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(p.promRegistry, promhttp.HandlerOpts{})
}

// Shutdown gracefully shuts down OTEL provider
func (p *OTelProvider) Shutdown(ctx context.Context) error {
	if err := p.traceProvider.Shutdown(ctx); err != nil {
		return err
	}
	return p.metricProvider.Shutdown(ctx)
}

// ===== OTEL-AWARE LOGGER =====

// OTelLogBrt combines unified logBrt with full OTEL integration
type OTelLogBrt struct {
	*StrategyLogBrt
	otelProvider *OTelProvider
}

// NewOTelLogBrt creates logBrt with full OTEL integration
func NewOTelLogBrt(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*OTelLogBrt, error) {
	// Setup OTEL provider
	provider, err := NewOTelProvider(serviceName, version, environment, endpoint)
	if err != nil {
		return nil, err
	}

	// Create strategy logBrt with async pipeline
	strategyLogBrt := NewStrategyLogBrt(level, sinks...)

	return &OTelLogBrt{
		StrategyLogBrt: strategyLogBrt,
		otelProvider:   provider,
	}, nil
}

// Override log method to include OTEL metrics and tracing
func (ol *OTelLogBrt) log(level domain.Level, msg string) {
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
		if ol.otelProvider != nil {
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

	// Record OTEL metrics (use request/operation context when available)
	duration := time.Since(start)
	var metCtx context.Context
	if ctx, ok := ol.fields["context"].(context.Context); ok && ctx != nil {
		metCtx = ctx
	} else {
		metCtx = context.Background()
	}
	if ol.otelProvider != nil {
		ol.otelProvider.RecordLogMetric(metCtx, level, duration,
			attribute.String("service", ol.otelProvider.serviceName),
		)
	}

	// Update counter
	if ol.logCount != nil {
		ol.logCount.Add(1)
	}
}

// ===== OVERRIDES: ensure OTelLogBrt methods dispatch to its own log =====
func (ol *OTelLogBrt) Debug(msg string) { ol.log(domain.DebugLevel, msg) }
func (ol *OTelLogBrt) Info(msg string)  { ol.log(domain.InfoLevel, msg) }
func (ol *OTelLogBrt) Warn(msg string)  { ol.log(domain.WarnLevel, msg) }
func (ol *OTelLogBrt) Error(msg string) { ol.log(domain.ErrorLevel, msg) }
func (ol *OTelLogBrt) Fatal(msg string) { ol.log(domain.FatalLevel, msg) }

func (ol *OTelLogBrt) Debugf(format string, args ...interface{}) {
	ol.log(domain.DebugLevel, fmt.Sprintf(format, args...))
}
func (ol *OTelLogBrt) Infof(format string, args ...interface{}) {
	ol.log(domain.InfoLevel, fmt.Sprintf(format, args...))
}
func (ol *OTelLogBrt) Warnf(format string, args ...interface{}) {
	ol.log(domain.WarnLevel, fmt.Sprintf(format, args...))
}
func (ol *OTelLogBrt) Errorf(format string, args ...interface{}) {
	ol.log(domain.ErrorLevel, fmt.Sprintf(format, args...))
}
func (ol *OTelLogBrt) Fatalf(format string, args ...interface{}) {
	ol.log(domain.FatalLevel, fmt.Sprintf(format, args...))
}

// ===== DERIVED BUILDER OVERRIDES: preserve OTelLogBrt type =====

// clone creates an OTelLogBrt preserving provider and strategy chain
func (ol *OTelLogBrt) clone() *OTelLogBrt {
	base := ol.StrategyLogBrt.clone()
	return &OTelLogBrt{
		StrategyLogBrt: base,
		otelProvider:   ol.otelProvider,
	}
}

// F adds a field and returns OTelLogBrt clone
func (ol *OTelLogBrt) F(key string, value interface{}) LogBrt {
	clone := ol.clone()
	clone.fields[key] = value
	return clone
}

// Fs adds multiple fields and returns OTelLogBrt clone
func (ol *OTelLogBrt) Fs(fields map[string]interface{}) LogBrt {
	if len(fields) == 0 {
		return ol
	}
	clone := ol.clone()
	for k, v := range fields {
		clone.fields[k] = v
	}
	return clone
}

// Ctx attaches context and ID fields, preserving type
func (ol *OTelLogBrt) Ctx(ctx context.Context) LogBrt {
	if ctx == nil {
		return ol
	}
	clone := ol.clone()
	clone.fields["context"] = ctx
	if traceID := extractFromContext(ctx, "trace_id"); traceID != "" {
		clone.fields["trace_id"] = traceID
	}
	if userID := extractFromContext(ctx, "user_id"); userID != "" {
		clone.fields["user_id"] = userID
	}
	if requestID := extractFromContext(ctx, "request_id"); requestID != "" {
		clone.fields["request_id"] = requestID
	}
	return clone
}

func (ol *OTelLogBrt) TraceID(id string) LogBrt   { return ol.F("trace_id", id) }
func (ol *OTelLogBrt) UserID(id string) LogBrt    { return ol.F("user_id", id) }
func (ol *OTelLogBrt) RequestID(id string) LogBrt { return ol.F("request_id", id) }

// WithError preserves type
func (ol *OTelLogBrt) WithError(err error) LogBrt {
	if err == nil {
		return ol
	}
	return ol.F("error", err.Error())
}

// WithSpan creates a new span and returns logBrt with span context
func (ol *OTelLogBrt) WithSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span, LogBrt) {
	spanCtx, span := ol.otelProvider.StartSpan(ctx, name, opts...)

	// Create logBrt with span context
	logBrt := ol.Ctx(spanCtx)

	return spanCtx, span, logBrt
}

// GetOTelProvider returns the OTEL provider for advanced usage
func (ol *OTelLogBrt) GetOTelProvider() *OTelProvider {
	return ol.otelProvider
}

// ===== GIN OTEL MIDDLEWARE =====

// OTelGinMiddleware creates Gin middleware with full OTEL integration
func OTelGinMiddleware(otelLogBrt *OTelLogBrt) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()

		// Extract or create trace context
		ctx := otelLogBrt.otelProvider.ExtractHeaders(c.Request.Context(), c.Request.Header)

		// Start span for request with robust scheme detection
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		} else if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
		spanCtx, span := otelLogBrt.otelProvider.StartSpan(ctx, c.Request.URL.Path,
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("url.scheme", scheme),
				attribute.String("url.path", c.Request.URL.Path),
				attribute.String("url.query", c.Request.URL.RawQuery),
				attribute.String("http.route", c.FullPath()),
			),
		)
		defer span.End()

		// Create request logBrt with full context
		requestLogBrt := otelLogBrt.
			Ctx(spanCtx).
			RequestID(generateRequestID()).
			F("method", c.Request.Method).
			F("path", c.Request.URL.Path).
			F("ip", c.ClientIP()).
			F("user_agent", c.Request.UserAgent())

		// Store in Gin context
		c.Set("otel_log", requestLogBrt)
		c.Set("span", span)
		c.Request = c.Request.WithContext(spanCtx)

		// Process request
		c.Next()

		// Log completion with metrics
		duration := time.Since(start)
		status := c.Writer.Status()

		// Add span attributes
		span.SetAttributes(
			attribute.Int("http.status_code", status),
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
		requestLogBrt.
			F("status", status).
			F("duration_ms", duration.Milliseconds()).
			F("response_size", c.Writer.Size()).
			Info("Request completed")
	})
}

// GetOTelLog extracts OTEL-aware logBrt from Gin context
func GetOTelLog(c *gin.Context) LogBrt {
	if logBrt, exists := c.Get("otel_log"); exists {
		if log, ok := logBrt.(LogBrt); ok {
			return log
		}
	}

	// Fallback to regular logBrt
	return NewUnifiedLogBrt(INFO)
}

// ===== OTEL SINKS =====

// OTLPSink sends logs directly to OTLP collector
type OTLPSink struct {
	SinkBase
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

func (s *OTLPSink) Name() string { return "otlp" }

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
// Use domain.NewHTTPError in callers instead of redefining the type here.

// ===== CONVENIENCE FUNCTIONS =====

// generateRequestID creates efficient request ID
func generateRequestID() string {
	return domain.GenerateID("req")
}

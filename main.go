package main

import (
	"context"
	"fmt"
	"os"
	"time"

	adapIn "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/inbound"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
	ports "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/outbound"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/shared"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel/trace"
)

// Re-export types from ports for public API
type (
	// Core types from domain
	Level           = domain.Level
	Field           = domain.Field
	LogEntry        = domain.LogEntry
	StructuredError = domain.StructuredError
	FilterRule      = domain.FilterRule
	ResponseOption  = domain.ResponseOption

	// Inbound ports
	Logger        = inbound.Logger
	Simple        = inbound.Simple
	Trace         = inbound.Trace
	Response      = inbound.Response
	Service       = inbound.Service
	Features      = inbound.Features
	ErrCategories = inbound.ErrCategories
	Feature       = inbound.Feature
	ErrHandler    = inbound.ErrHandler
	Config        = inbound.Config

	// Outbound ports
	Sink       = ports.Sink
	ConfigSrc  = ports.ConfigSrc
	Metrics    = ports.Metrics
	TraceSrc   = ports.TraceSrc
	Filter     = ports.Filter
	Sampler    = ports.Sampler
	Formatter  = ports.Formatter
	SinkConfig = ports.SinkConfig

	// Helpers from inbound simple logger
	GinLogger       = adapIn.GinLogger
	ResponseOptions = adapIn.ResponseOptions
	Parent          = adapIn.Parent

	// AMQP types
	AMQPHandler       = shared.AMQPHandler
	AMQPBatchHandler  = shared.AMQPBatchHandler
	AMQPPublisher     = shared.AMQPPublisher
	AMQPConsumer      = shared.AMQPConsumer
	AMQPBatchConsumer = shared.AMQPBatchConsumer
	OTelProvider      = shared.OTelProvider

	// Metrics helper
	SafePrometheus = shared.SafePrometheus

	// Struct logger helper
	StructLogger = shared.StructLogger
)

// Re-export log levels
const (
	DebugLevel = domain.DebugLevel
	InfoLevel  = domain.InfoLevel
	WarnLevel  = domain.WarnLevel
	ErrorLevel = domain.ErrorLevel
	FatalLevel = domain.FatalLevel
)

// Option configures the logger
type Option = adapIn.Option

// Standard options
var (
	WithLevel = adapIn.WithLevel
	WithSinks = adapIn.WithSinks
)

// Factory functions

// NewLogger creates a new logger with adapters
func NewLogger(opts ...Option) (Logger, error) { return adapIn.NewZapLoggerAdapter(opts...) }

// NewSafeLogger wraps a logger to prevent panics
func NewSafeLogger(logger Logger) Logger { return adapIn.NewSafeLoggerAdapter(logger) }

// Sink factory functions

// NewStdoutSink creates a stdout sink
func NewStdoutSink() Sink { return outbound.NewStdoutSink(outbound.NewJSONFormatter()) }

// NewFileSink creates a file sink with rotation
func NewFileSink(filename string, maxSize, maxAge, maxBackups int, compress bool) Sink {
	return outbound.NewFileSink(filename, maxSize, maxAge, maxBackups, compress, outbound.NewJSONFormatter())
}

// NewHTTPSink creates an HTTP sink
func NewHTTPSink(url string, headers map[string]string, batchSize int) Sink {
	return outbound.NewHTTPSink(url, headers, outbound.NewDefaultHTTPClient(), batchSize)
}

// NewBufferedSink creates a buffered sink
func NewBufferedSink(sink Sink, bufferSize int, flushInterval time.Duration) Sink {
	return outbound.NewBufferedSink(sink, bufferSize, flushInterval)
}

// NewMultiplexSink creates a multiplex sink
func NewMultiplexSink(sinks ...Sink) Sink { return outbound.NewMultiplexSink(sinks...) }

// NewOTLPSink creates an OTLP sink
func NewOTLPSink(endpoint string, insecure bool) (Sink, error) {
	return outbound.NewOTLPSink(endpoint, insecure)
}

// NewLokiSink creates a Loki sink
func NewLokiSink(url string, labels map[string]string, batchSize int) Sink {
	return outbound.NewLokiSink(url, labels, batchSize)
}

// Formatter factory functions

// NewJSONFormatter creates a JSON formatter
func NewJSONFormatter() Formatter { return outbound.NewJSONFormatter() }

// NewTextFormatter creates a text formatter
func NewTextFormatter() Formatter { return outbound.NewTextFormatter() }

// NewLogfmtFormatter creates a logfmt formatter
func NewLogfmtFormatter() Formatter { return outbound.NewLogfmtFormatter() }

// Configuration providers

// NewRedisConfigProvider creates a Redis-based config provider
func NewRedisConfigProvider(addr, password string, db int, keyPrefix string) (ConfigSrc, error) {
	return outbound.NewRedisConfigProvider(addr, password, db, keyPrefix)
}

// Metrics providers

// NewPrometheusMetricsProvider creates a Prometheus metrics provider
func NewPrometheusMetricsProvider(serviceName string) (Metrics, error) {
	return outbound.NewPrometheusMetricsProvider(serviceName)
}

// NewSimpleMetricsProvider creates a simple in-memory metrics provider
func NewSimpleMetricsProvider() Metrics { return outbound.NewSimpleMetricsProvider() }

// Trace providers

// NewOTelTraceProvider creates an OpenTelemetry trace provider
func NewOTelTraceProvider(serviceName, endpoint string, insecure bool) (TraceSrc, error) {
	return outbound.NewOTelTraceProvider(serviceName, endpoint, insecure)
}

// NewNoOpTraceProvider creates a no-op trace provider
func NewNoOpTraceProvider() TraceSrc { return outbound.NewNoOpTraceProvider() }

// Re-export Gin helpers for external usage
func NewGinEngine() *gin.Engine { return adapIn.NewGinEngine() }

// Re-export middleware and helpers
func GinMiddleware(logger Logger) gin.HandlerFunc           { return adapIn.GinMiddleware(logger) }
func GetLoggerFromGinContext(c *gin.Context) (Logger, bool) { return adapIn.GetLoggerFromGinContext(c) }
func GetLogFrmGin(c *gin.Context, operationName string) GinLogger {
	return adapIn.GetLogFrmGin(c, operationName)
}
func OptsResponse() *ResponseOptions { return adapIn.OptsResponse() }

// OTel provider wrappers

func NewOTelProvider(serviceName, endpoint string, insecure bool) (*OTelProvider, error) {
	return shared.NewOTelProvider(serviceName, endpoint, insecure)
}

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return shared.StartSpan(ctx, name, opts...)
}

func StartSpanWithLogger(ctx context.Context, logger Logger, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span, Logger) {
	return shared.StartSpanWithLogger(ctx, logger, name, opts...)
}

func StartFlatSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return shared.StartFlatSpan(ctx, name, opts...)
}

func RecordError(span trace.Span, err error, msg string) { shared.RecordError(span, err, msg) }
func PrometheusHandler() interface{}                     { return shared.PrometheusHandler() }

// AMQP re-exports
func NewAMQPConsumer(ch *amqp.Channel, logger Logger, provider *OTelProvider) *AMQPConsumer {
	return shared.NewAMQPConsumer(ch, logger, provider)
}
func NewAMQPPublisher(ch *amqp.Channel, logger Logger, provider *OTelProvider) *AMQPPublisher {
	return shared.NewAMQPPublisher(ch, logger, provider)
}
func NewAMQPBatchConsumer(ch *amqp.Channel, logger Logger, provider *OTelProvider, batchSize int, timeout time.Duration) *AMQPBatchConsumer {
	return shared.NewAMQPBatchConsumer(ch, logger, provider, batchSize, timeout)
}
func DeadLetterHandler(h AMQPHandler, pub *AMQPPublisher, dlx, dlrk string, maxRetries int) AMQPHandler {
	return shared.DeadLetterHandler(h, pub, dlx, dlrk, maxRetries)
}
func InjectAMQPHeaders(ctx context.Context, headers map[string]interface{}) {
	shared.InjectAMQPHeaders(ctx, headers)
}
func ExtractAMQPHeaders(ctx context.Context, headers map[string]interface{}) context.Context {
	return shared.ExtractAMQPHeaders(ctx, headers)
}

// Struct tag helpers
func ExtractFields(v interface{}) map[string]interface{}  { return shared.ExtractFields(v) }
func LogStructFields(logger Logger, v interface{}) Logger { return shared.LogStructFields(logger, v) }
func NewStructLogger(logger Logger) *StructLogger         { return shared.NewStructLogger(logger) }
func NewSafePrometheusWith(subsystem, service, environment string, enabled bool) *SafePrometheus {
	return shared.NewSafePrometheusWith(subsystem, service, environment, enabled)
}

// Testing helpers
func NewMockLogger() Logger { return shared.NewMockLogger() }

// Context helpers
func GetLoggerFromContext(ctx context.Context) (Logger, bool) {
	if v := ctx.Value("logger"); v != nil {
		if l, ok := v.(Logger); ok {
			return l, true
		}
	}
	return nil, false
}

// Global logger instance
var globalLogger Logger

func init() {
	var err error
	globalLogger, err = NewLogger(WithLevel(InfoLevel), WithSinks(NewStdoutSink()))
	if err != nil {
		panic(fmt.Sprintf("failed to initialize global logger: %v", err))
	}
	globalLogger = NewSafeLogger(globalLogger)
}

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger Logger) {
	if logger != nil {
		globalLogger = NewSafeLogger(logger)
	}
}

// GetGlobalLogger returns the global logger
func GetGlobalLogger() Logger { return globalLogger }

// Convenience logging functions using global logger
func Debug(msg string, fields ...Field) { globalLogger.Debug(msg, fields...) }
func Info(msg string, fields ...Field)  { globalLogger.Info(msg, fields...) }
func Warn(msg string, fields ...Field)  { globalLogger.Warn(msg, fields...) }
func Error(msg string, fields ...Field) { globalLogger.Error(msg, fields...) }
func Fatal(msg string, fields ...Field) { globalLogger.Fatal(msg, fields...) }

// WithContext returns logger with context
func WithContext(ctx context.Context) Logger { return globalLogger.Ctx(ctx) }

// WithField returns logger with field
func WithField(key string, value interface{}) Logger { return globalLogger.F(key, value) }

// WithFields returns logger with fields
func WithFields(fields map[string]interface{}) Logger { return globalLogger.Fs(fields) }

// WithError returns logger with error
func WithError(err error) Logger { return globalLogger.Err(err) }

// Use case functions

func LogWithContext(ctx context.Context, level Level, msg string, fields map[string]interface{}) error {
	l := globalLogger.Ctx(ctx).F("level", level).F("message", msg)
	if len(fields) > 0 {
		l = l.Fs(fields)
	}
	switch level {
	case DebugLevel:
		l.Debug(msg)
	case InfoLevel:
		l.Info(msg)
	case WarnLevel:
		l.Warn(msg)
	case ErrorLevel:
		l.Error(msg)
	case FatalLevel:
		l.Fatal(msg)
	}
	return nil
}

func LogError(ctx context.Context, err error, category string, details map[string]interface{}) error {
	globalLogger.Ctx(ctx).Err(err).F("error_category", category).Fs(details).Error("Error occurred")
	return nil
}

func LogStructuredError(ctx context.Context, err *StructuredError) error {
	globalLogger.Ctx(ctx).F("error_code", err.Code).F("error_category", err.Category).F("error_message", err.Message).Fs(err.Details).Error(err.Error())
	return nil
}

func ParseLevel(level string) Level { return domain.ParseLevelString(level) }
func GenerateRequestID() string     { return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), os.Getpid()) }

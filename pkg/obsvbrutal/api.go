// Package obsvbrutal provides the public API for the Log Brutal logging system
package obsvbrutal

import (
	"context"
	"fmt"

	adapIn "obs-brutal/pkg/obsvbrutal/adapters/inbound"
	"obs-brutal/pkg/obsvbrutal/adapters/outbound"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/inbound"
	ports "obs-brutal/pkg/obsvbrutal/core/ports/outbound"
	"obs-brutal/pkg/obsvbrutal/core/usecases"
	"os"
	"time"

	"github.com/samber/lo"
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
func NewLogger(opts ...Option) (Logger, error) {
	return adapIn.NewZapLoggerAdapter(opts...)
}

// NewSafeLogger wraps a logger to prevent panics
func NewSafeLogger(logger Logger) Logger {
	return adapIn.NewSafeLoggerAdapter(logger)
}

// Sink factory functions

// NewStdoutSink creates a stdout sink
func NewStdoutSink() Sink {
	return outbound.NewStdoutSink(outbound.NewJSONFormatter())
}

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
func NewMultiplexSink(sinks ...Sink) Sink {
	return outbound.NewMultiplexSink(sinks...)
}

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
func NewJSONFormatter() Formatter {
	return outbound.NewJSONFormatter()
}

// NewTextFormatter creates a text formatter
func NewTextFormatter() Formatter {
	return outbound.NewTextFormatter()
}

// NewLogfmtFormatter creates a logfmt formatter
func NewLogfmtFormatter() Formatter {
	return outbound.NewLogfmtFormatter()
}

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
func NewSimpleMetricsProvider() Metrics {
	return outbound.NewSimpleMetricsProvider()
}

// Trace providers

// NewOTelTraceProvider creates an OpenTelemetry trace provider
func NewOTelTraceProvider(serviceName, endpoint string, insecure bool) (TraceSrc, error) {
	return outbound.NewOTelTraceProvider(serviceName, endpoint, insecure)
}

// NewNoOpTraceProvider creates a no-op trace provider
func NewNoOpTraceProvider() TraceSrc {
	return outbound.NewNoOpTraceProvider()
}

// Global logger instance
var globalLogger Logger

func init() {
	// Initialize with safe default logger
	var err error
	globalLogger, err = NewLogger(
		WithLevel(InfoLevel),
		WithSinks(NewStdoutSink()),
	)
	if err != nil {
		// This should never happen with default config
		panic(fmt.Sprintf("failed to initialize global logger: %v", err))
	}

	// Make it safe
	globalLogger = NewSafeLogger(globalLogger)
}

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger Logger) {
	if logger == nil {
		return
	}
	globalLogger = NewSafeLogger(logger)
}

// GetGlobalLogger returns the global logger
func GetGlobalLogger() Logger {
	return globalLogger
}

// Convenience logging functions using global logger

// Debug logs debug message
func Debug(msg string, fields ...Field) {
	globalLogger.Debug(msg, fields...)
}

// Info logs info message
func Info(msg string, fields ...Field) {
	globalLogger.Info(msg, fields...)
}

// Warn logs warning message
func Warn(msg string, fields ...Field) {
	globalLogger.Warn(msg, fields...)
}

// Error logs error message
func Error(msg string, fields ...Field) {
	globalLogger.Error(msg, fields...)
}

// Fatal logs fatal message
func Fatal(msg string, fields ...Field) {
	globalLogger.Fatal(msg, fields...)
}

// WithContext returns logger with context
func WithContext(ctx context.Context) Logger {
	return globalLogger.Ctx(ctx)
}

// WithField returns logger with field
func WithField(key string, value interface{}) Logger {
	return globalLogger.F(key, value)
}

// WithFields returns logger with fields
func WithFields(fields map[string]interface{}) Logger {
	return globalLogger.Fs(fields)
}

// WithError returns logger with error
func WithError(err error) Logger {
	return globalLogger.Err(err)
}

// Use case functions

// DefaultLoggingUseCase provides access to the logging use case
var DefaultLoggingUseCase *usecases.LoggingUseCase

// InitializeUseCases initializes the use cases
// TODO: Fix interface compatibility between inbound and ports packages
func InitializeUseCases(
	featureRegistry Features,
	errorCategoryRegistry ErrCategories,
	configProvider ConfigSrc,
	metricsProvider interface{},
) {
	// Temporarily disabled due to interface incompatibility
	// Will be fixed in next iteration
	/*
		DefaultLoggingUseCase = usecases.NewLoggingUseCase(
			globalLogger,
			featureRegistry,
			errorCategoryRegistry,
			configProvider,
			metricsProvider,
		)
	*/
}

// LogWithContext logs with context and features
func LogWithContext(ctx context.Context, level Level, msg string, fields map[string]interface{}) error {
	if DefaultLoggingUseCase == nil {
		// Use global logger directly
		l := globalLogger.Ctx(ctx).
			F("level", level).
			F("message", msg)
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

	return DefaultLoggingUseCase.LogWithContext(ctx, level, msg, fields)
}

// LogError logs an error with category
func LogError(ctx context.Context, err error, category string, details map[string]interface{}) error {
	if DefaultLoggingUseCase == nil {
		// Use global logger directly
		globalLogger.Ctx(ctx).
			Err(err).
			F("error_category", category).
			Fs(details).
			Error("Error occurred")
		return nil
	}

	return DefaultLoggingUseCase.LogErr(ctx, err, category, details)
}

// LogStructuredError logs a structured error
func LogStructuredError(ctx context.Context, err *StructuredError) error {
	if DefaultLoggingUseCase == nil {
		// Use global logger directly
		globalLogger.Ctx(ctx).
			F("error_code", err.Code).
			F("error_category", err.Category).
			F("error_message", err.Message).
			Fs(err.Details).
			Error(err.Error())
		return nil
	}

	return DefaultLoggingUseCase.LogStruct(ctx, lo.FromPtr(err))
}

// Helper functions

// ParseLevel converts string to Level
func ParseLevel(level string) Level {
	switch level {
	case "debug", "DEBUG":
		return DebugLevel
	case "info", "INFO":
		return InfoLevel
	case "warn", "WARN", "warning", "WARNING":
		return WarnLevel
	case "error", "ERROR":
		return ErrorLevel
	case "fatal", "FATAL":
		return FatalLevel
	default:
		return InfoLevel
	}
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), os.Getpid())
}

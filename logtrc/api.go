// Package logtrc provides the simplest, fastest, and most efficient logging API
package logtrc

import (
	"context"
	"fmt"
	"time"

	"obs-brutal/internal/core"
	"obs-brutal/internal/core/domain"

	"github.com/gin-gonic/gin"
)

// ===== SIMPLIFIED CORE TYPES =====

// Level represents log levels (simplified)
type Level = domain.Level

const (
	DEBUG Level = domain.DebugLevel
	INFO  Level = domain.InfoLevel
	WARN  Level = domain.WarnLevel
	ERROR Level = domain.ErrorLevel
	FATAL Level = domain.FatalLevel
)

// Logger is the main interface - simplified and powerful
type Logger = core.Logger

// LogTrc combines logging + tracing + response building
type LogTrc = core.LogTrc

// Tracer for hierarchical tracing
type Tracer = core.Tracer

// Response builder types
type (
	SimpleResponseBuilder = core.SimpleResponseBuilder
	ResponseOptions       = core.ResponseOptions
	SimpleResponseOption  = core.SimpleResponseOption
)

// Simple is the ultra-easy interface for web apps
type Simple interface {
	// Super simple logging
	Log(msg string)
	Logf(format string, args ...interface{})

	// With single field
	With(key string, value interface{}) Simple

	// With error
	Error(err error) Simple

	// Close when done
	Close()
}

// ===== UNIFIED SMART FACTORY =====

// New creates smart logger with options (no environment variables!)
// Usage examples:
//
//	log := logtrc.New()                                    // Simple mode (762k+ logs/sec)
//	log := logtrc.New(Masking(true))                       // With PII masking
//	log := logtrc.New(OTel("jaeger:14268"), Masking(true)) // Full enterprise
func New(opts ...ConfigOption) Logger {
	return core.NewSmartLoggerWithOptions(opts...)
}

// NewDefault creates logger with sensible defaults
func NewDefault() Logger {
	return New()
}

// ===== OPTIONS (Replace Environment Variables) =====

// Configuration options
var (
	SrvName    = core.SrvName    // Service name
	Version    = core.Version    // Service version
	Env        = core.Env        // Environment
	OTel       = core.OTel       // OTEL endpoint
	Prometheus = core.Prometheus // Prometheus endpoint
	Loki       = core.Loki       // Loki endpoint
	Promtail   = core.Promtail   // Promtail endpoint
	Masking    = core.Masking    // Enable PII masking
	Async      = core.Async      // Enable async pipeline
	LogLevel   = core.LogLevel   // Set log level
)

// Response options
var (
	Opts = core.NewResponseOpts() // Response options factory
)

// ConfigOption type alias
type ConfigOption = core.ConfigOption

// ===== LOGTRC INTEGRATION =====

// GetLogTrcFrmGin gets LogTrc from Gin context with options
// Automatically binds TraceID, SpanID and connects to observability stack
// Usage: ltrace := logtrc.GetLogTrcFrmGin(c, "GetUsers", OTel("jaeger:14268"), Masking(true))
func GetLogTrcFrmGin(c *gin.Context, operation string, opts ...ConfigOption) LogTrc {
	return core.NewSmartLogTrc(c, operation, opts...)
}

// OptsResponse creates response options factory
func OptsResponse() *ResponseOptions {
	return core.OptsResponse()
}

// GlobalDetector provides access to capability detection
var GlobalDetector = core.GlobalDetector

// ===== BACKWARD COMPATIBILITY (Optional Modes) =====

// NewWeb creates logger optimized for web applications
func NewWeb(serviceName string) Logger {
	logger := New()
	return logger.F("service", serviceName)
}

// NewSimple creates the simplest possible logger for basic usage
func NewSimple(ctx context.Context, operation string) Simple {
	logger := New().
		Ctx(ctx).
		F("operation", operation)

	return &simpleLogger{logger: logger}
}

// ===== GIN INTEGRATION (SIMPLIFIED) =====

// Middleware creates Gin middleware (simplified)
func Middleware(serviceName string) gin.HandlerFunc {
	logger := NewWeb(serviceName)
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()

		// Auto-add request context
		log := logger.
			RequestID(generateID()).
			F("method", c.Request.Method).
			F("path", c.Request.URL.Path).
			F("ip", c.ClientIP())

		// Store in context
		c.Set("log", log)
		c.Next()

		// Log completion
		log.F("status", c.Writer.Status()).
			F("duration_ms", time.Since(start).Milliseconds()).
			Info("Request completed")
	})
}

// GetLog extracts logger from Gin context (simplified)
func GetLog(c *gin.Context) Logger {
	if logger, exists := c.Get("log"); exists {
		if log, ok := logger.(Logger); ok {
			return log
		}
	}
	return NewDefault()
}

// GetSimple gets simple logger from Gin context (backward compatibility)
func GetSimple(c *gin.Context, operation string) Simple {
	logger := GetLog(c).F("operation", operation)
	return &simpleLogger{logger: logger}
}

// ===== ADVANCED MIDDLEWARE =====

// OTelMiddleware creates Gin middleware with full OTEL integration
func OTelMiddleware(otelLogger *core.OTelLogger) gin.HandlerFunc {
	return core.OTelGinMiddleware(otelLogger)
}

// ===== STRATEGY UTILITIES =====

// CreateLevelFilter creates level-based filter
func CreateLevelFilter(minLevel, maxLevel Level) core.FilterStrategy {
	return core.NewLevelFilter(minLevel, maxLevel)
}

// CreateRateSampler creates rate-based sampler
func CreateRateSampler(rate float64) core.SamplerStrategy {
	return core.NewRateSampler(rate)
}

// CreateAdaptiveSampler creates adaptive sampler
func CreateAdaptiveSampler(baseRate, minRate, maxRate float64) core.SamplerStrategy {
	return core.NewAdaptiveSampler(baseRate, minRate, maxRate)
}

// CreatePIIMasker creates PII masking strategy
func CreatePIIMasker() core.MaskingStrategy {
	return core.NewPIIMasker()
}

// ===== CORRELATION UTILITIES =====

// Fs extracts and masks fields in one line (replaces ExtractFields)
// Usage: ltrace.Fs(user).Prt("User created") - extracts and masks automatically
func Fs(data interface{}) map[string]interface{} {
	extractor := &core.ExtractOptions{}
	return extractor.Fields(data)
}

// ===== IMPLEMENTATIONS =====

// simpleLogger implements Simple interface
type simpleLogger struct {
	logger Logger
}

func (s *simpleLogger) Log(msg string) {
	s.logger.Info(msg)
}

func (s *simpleLogger) Logf(format string, args ...interface{}) {
	s.logger.Infof(format, args...)
}

func (s *simpleLogger) With(key string, value interface{}) Simple {
	return &simpleLogger{logger: s.logger.F(key, value)}
}

func (s *simpleLogger) Error(err error) Simple {
	return &simpleLogger{logger: s.logger.WithError(err)}
}

func (s *simpleLogger) Close() {
	// Nothing to do - logger handles cleanup
}

// ===== HELPER FUNCTIONS =====

// buildOptimalSinks creates the best sinks for given outputs
func buildOptimalSinks(outputs ...string) []core.Sink {
	if len(outputs) == 0 {
		outputs = []string{"stdout"}
	}

	var sinks []core.Sink
	for _, output := range outputs {
		switch output {
		case "stdout", "console":
			sinks = append(sinks, core.NewFastStdoutSink())
		case "file":
			// Auto-configure optimal file sink
			sinks = append(sinks, core.NewOptimalFileSink())
		case "buffer":
			// Auto-configure optimal buffered sink
			sinks = append(sinks, core.NewBufferedSink())
		}
	}

	return sinks
}

// generateID creates simple request ID
func generateID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// ===== GLOBAL CONVENIENCE FUNCTIONS =====

var defaultLogger = NewDefault()

// Quick global logging functions
func Debug(msg string) { defaultLogger.Debug(msg) }
func Info(msg string)  { defaultLogger.Info(msg) }
func Warn(msg string)  { defaultLogger.Warn(msg) }
func Error(msg string) { defaultLogger.Error(msg) }
func Fatal(msg string) { defaultLogger.Fatal(msg) }

func Debugf(format string, args ...interface{}) { defaultLogger.Debugf(format, args...) }
func Infof(format string, args ...interface{})  { defaultLogger.Infof(format, args...) }
func Warnf(format string, args ...interface{})  { defaultLogger.Warnf(format, args...) }
func Errorf(format string, args ...interface{}) { defaultLogger.Errorf(format, args...) }
func Fatalf(format string, args ...interface{}) { defaultLogger.Fatalf(format, args...) }

// With fields
func With(key string, value interface{}) Logger       { return defaultLogger.F(key, value) }
func WithFields(fields map[string]interface{}) Logger { return defaultLogger.Fs(fields) }
func WithError(err error) Logger                      { return defaultLogger.WithError(err) }
func WithContext(ctx context.Context) Logger          { return defaultLogger.Ctx(ctx) }

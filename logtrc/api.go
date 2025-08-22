// for applications with optional observability integrations (OTEL, Prometheus).
//
// It exposes a small, stable public surface that wraps the internal/core package
// with sensible defaults and clean, fluent methods.
package logtrc

import (
	"context"
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

// LogBrt is the main interface for structured logging with fluent APIs.
type LogBrt = core.LogBrt

// Log is a short alias for LogBrt (recommended concise type name).
type Log = core.LogBrt

// Sink is the sink interface exposed for output targets.
type Sink = core.Sink

// LogTrc combines logging + tracing + response building for web apps.
// Obtain it via GetLogTrcFrmGin and close with .Close() when done.
type LogTrc = core.LogTrc

// Tracer represents hierarchical traces. Create via LogTrc.FlatPr/ChildPr.
type Tracer = core.Tracer

// Response builder types
type (
	// SimpleResponseBuilder builds and sends JSON responses with auto-logging.
	SimpleResponseBuilder = core.SimpleResponseBuilder
)

// OTelLogBrt is the OpenTelemetry-enabled logBrt type.
type OTelLogBrt = core.OTelLogBrt

// AsyncLogBrt exposes async logBrt type.
type AsyncLogBrt = core.AsyncLogBrt

// SecurityLogBrt exposes enterprise security logBrt type.
type SecurityLogBrt = core.SecurityLogBrt

// NewOTelLogBrt creates an OTEL-enabled logBrt quickly from logtrc.
//
// Example:
//
//	l, err := logtrc.NewOTelLogBrt(
//	    logtrc.SrvName("svc"), logtrc.Version("1.0.0"), logtrc.Env("prod"),
//	    "jaeger:4317", logtrc.INFO,
//	)
//	if err != nil { panic(err) }
func NewOTelLogBrt(serviceName, version, environment, endpoint string, level Level, sinks ...core.Sink) (*OTelLogBrt, error) {
	return core.NewOTelLogBrt(serviceName, version, environment, endpoint, level, sinks...)
}

// NewAsyncLogBrt creates logBrt with async pipeline.
func NewAsyncLogBrt(level Level, sinks ...Sink) *AsyncLogBrt {
	return core.NewAsyncLogBrt(level, sinks...)
}

// NewSecurityLogBrt creates OTEL logBrt with security features enabled.
func NewSecurityLogBrt(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*SecurityLogBrt, error) {
	return core.NewSecurityLogBrt(serviceName, version, environment, endpoint, level, sinks...)
}

// Sinks
func NewFastStdoutSink() Sink  { return core.NewFastStdoutSink() }
func NewOptimalFileSink() Sink { return core.NewOptimalFileSink() }
func NewJSONSink() Sink        { return core.NewJSONSink() }
func NewBufferedSink() Sink    { return core.NewBufferedSink() }
func NewBufferedSinkWith(size int, timeout time.Duration) Sink {
	return core.NewBufferedSinkWith(size, timeout)
}

// ===== UNIFIED SMART FACTORY =====

// New creates a smart logBrt with options (no environment variables).
// Example:
//
//	log := logtrc.New(logtrc.SrvName("svc"), logtrc.Masking(true))
func New(opts ...ConfigOption) LogBrt {
	return core.NewSmartLogBrtWithOptions(opts...)
}

// NewDefault creates logBrt with sensible defaults.
func NewDefault() LogBrt { return New() }

// ===== OPTIONS (Replace Environment Variables) =====

// Public option helpers for clean configuration
var (
	SrvName    = core.SrvName
	Version    = core.Version
	Env        = core.Env
	OTel       = core.OTel
	Prometheus = core.Prometheus
	Loki       = core.Loki
	Promtail   = core.Promtail
	Masking    = core.Masking
	Async      = core.Async
	LogLevel   = core.LogLevel
)

// Response options factory
var (
	Opts = core.NewResponseOpts()
)

// ConfigOption configures the logBrt and integrations.
type ConfigOption = core.ConfigOption

// ===== LOGTRC INTEGRATION =====

// GetLogTrcFrmGin gets LogTrc from Gin context with options.
// It auto-binds TraceID/SpanID and connects to the observability stack.
func GetLogTrcFrmGin(c *gin.Context, operation string, opts ...ConfigOption) LogTrc {
	return core.NewSmartLogTrc(c, operation, opts...)
}

// OptsResponse creates response options factory for chainable helpers.
// Deprecated: use Opts (NewResponseOpts) instead.
func OptsResponse() *core.ResponseOpts { return core.NewResponseOpts() }

// GlobalDetector exposes capability detection (optional usage).
var GlobalDetector = core.GlobalDetector

// ===== BACKWARD COMPATIBILITY (Optional Modes) =====

// NewWeb creates logBrt optimized for web applications (adds service field).
func NewWeb(serviceName string) LogBrt {
	logBrt := New()
	return logBrt.F("service", serviceName)
}

// ===== GIN INTEGRATION (SIMPLIFIED) =====

// Middleware creates Gin middleware that propagates context and attaches logBrt.
func Middleware(serviceName string) gin.HandlerFunc {
	base := NewWeb(serviceName)
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()
		log := base.
			Ctx(c.Request.Context()).
			RequestID(domain.GenerateID("req")).
			F("method", c.Request.Method).
			F("path", c.Request.URL.Path).
			F("ip", c.ClientIP())

		c.Set("log", log)
		c.Next()

		log.F("status", c.Writer.Status()).
			F("duration_ms", time.Since(start).Milliseconds()).
			Info("Request completed")
	})
}

// GetLog extracts logBrt from Gin context (simplified)
func GetLog(c *gin.Context) LogBrt {
	if logBrt, exists := c.Get("log"); exists {
		if log, ok := logBrt.(LogBrt); ok {
			return log
		}
	}
	return NewDefault()
}

// GetOTelLog extracts OTEL-aware logBrt from Gin context.
func GetOTelLog(c *gin.Context) LogBrt {
	return core.GetOTelLog(c)
}

// ===== ADVANCED MIDDLEWARE =====

// OTelMiddleware creates Gin middleware with full OTEL integration.
func OTelMiddleware(otelLogBrt *core.OTelLogBrt) gin.HandlerFunc {
	return core.OTelGinMiddleware(otelLogBrt)
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
func CreatePIIMasker() core.MaskingStrategy { return core.NewPIIMasker() }

// ===== CORRELATION UTILITIES =====

// Fs extracts and masks fields in one line (replaces ExtractFields)
// Usage: ltrace.Fs(user).Prt("User created") - extracts and masks automatically
func Fs(data interface{}) map[string]interface{} {
	extractor := &core.ExtractOptions{}
	return extractor.Fields(data)
}

// ===== HELPER FUNCTIONS =====

// (no helpers currently)

// ===== GLOBAL CONVENIENCE FUNCTIONS =====

var defaultLogBrt = NewDefault()

func Debug(msg string) { defaultLogBrt.Debug(msg) }
func Info(msg string)  { defaultLogBrt.Info(msg) }
func Warn(msg string)  { defaultLogBrt.Warn(msg) }
func Error(msg string) { defaultLogBrt.Error(msg) }
func Fatal(msg string) { defaultLogBrt.Fatal(msg) }

func Debugf(format string, args ...interface{}) { defaultLogBrt.Debugf(format, args...) }
func Infof(format string, args ...interface{})  { defaultLogBrt.Infof(format, args...) }
func Warnf(format string, args ...interface{})  { defaultLogBrt.Warnf(format, args...) }
func Errorf(format string, args ...interface{}) { defaultLogBrt.Errorf(format, args...) }
func Fatalf(format string, args ...interface{}) { defaultLogBrt.Fatalf(format, args...) }

func With(key string, value interface{}) LogBrt       { return defaultLogBrt.F(key, value) }
func WithFields(fields map[string]interface{}) LogBrt { return defaultLogBrt.Fs(fields) }
func WithError(err error) LogBrt                      { return defaultLogBrt.WithError(err) }
func WithContext(ctx context.Context) LogBrt          { return defaultLogBrt.Ctx(ctx) }

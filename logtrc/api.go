// for applications with optional observability integrations (OTEL, Prometheus).
//
// It exposes a small, stable public surface that wraps the internal/core package
// with sensible defaults and clean, fluent methods.
package logtrc

import (
    "time"

	inboundlog "obs-brutal/internal/adapter/inbound/log"
	outboundotel "obs-brutal/internal/adapter/outbound/otel"
	alertsink "obs-brutal/internal/adapter/outbound/sink/alerts"
	cfgopts "obs-brutal/internal/adapter/outbound/sink/options"
	telem "obs-brutal/internal/adapter/outbound/telemetry"
	"obs-brutal/internal/core/domain"
    "obs-brutal/internal/core/port"
	service "obs-brutal/internal/core/service"
	secsvc "obs-brutal/internal/core/service/security"
	sfactory "obs-brutal/internal/shared"

	"github.com/gin-gonic/gin"
)

// ===== SIMPLIFIED CORE TYPES =====

// Level represents log levels exposed via the facade (alias of domain.Level).
type Level = domain.Level

const (
	DEBUG Level = domain.DebugLevel
	INFO  Level = domain.InfoLevel
	WARN  Level = domain.WarnLevel
	ERROR Level = domain.ErrorLevel
	FATAL Level = domain.FatalLevel
)

// LogBrt is the main interface for structured logging with fluent APIs.
// It is re-exported from the core service layer.
type LogBrt = service.LogBrt

// Sink is the output sink interface exposed by the facade.
type Sink = port.Sink

// LogTrc combines logging with a minimal response builder for web apps.
// Obtain it via GetLogTrcFrmGin.
type LogTrc struct {
    log LogBrt
    gin *gin.Context
}

// Prt prints a formatted message using the underlying logger (Info level).
func (lt *LogTrc) Prt(format string, args ...interface{}) { lt.log.Infof(format, args...) }

// GetTraceID extracts the trace ID from the bound Gin request context (if any).
func (lt *LogTrc) GetTraceID() string {
	if lt == nil || lt.gin == nil || lt.gin.Request == nil {
		return ""
	}
	if v := lt.gin.Request.Context().Value("trace_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// R builds a response using the inbound adapter's SimpleResponseBuilder and applies options.
func (lt *LogTrc) R(status int, opts ...cfgopts.ResponseOption) port.ResponseBuilder {
    rb := inboundlog.NewSimpleResponseBuilder(lt.gin, lt, status)
    for _, opt := range opts {
        if opt != nil {
            opt(rb)
        }
    }
    return rb
}

// OTelLogBrt is the OpenTelemetry-enabled logger type (re-export).
type OTelLogBrt = service.OTelLogBrt

// AsyncLogBrt is the async logger type (re-export).
type AsyncLogBrt = service.AsyncLogBrt

// SecurityLogBrt exposes enterprise security-enabled logger type.
type SecurityLogBrt = secsvc.SecurityLogBrt

// NewOTelLogBrt creates an OTEL-enabled logger via the outbound adapter.
//
// Example:
//
//	l, err := logtrc.NewOTelLogBrt(
//	    logtrc.SrvName("svc"), logtrc.Version("1.0.0"), logtrc.Env("prod"),
//	    "jaeger:4317", logtrc.INFO,
//	)
//	if err != nil { panic(err) }
func NewOTelLogBrt(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*OTelLogBrt, error) {
	return outboundotel.NewOTelLogBrt(serviceName, version, environment, endpoint, level, sinks...)
}

// NewOTel returns both logger and concrete provider (for metrics endpoints)
// NewOTel returns both the OTEL-enabled logger and its concrete provider.
func NewOTel(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*OTelLogBrt, *outboundotel.Provider, error) {
	return outboundotel.NewOTel(serviceName, version, environment, endpoint, level, sinks...)
}

// NewAsyncWithOTel constructs an async logger and wires its stats to OTEL metrics.
func NewAsyncWithOTel(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*AsyncLogBrt, error) {
	prov, err := outboundotel.NewOTelProvider(serviceName, version, environment, endpoint)
	if err != nil {
		return nil, err
	}
	tp, err := telem.NewOTelTelemetry(prov.ServiceName, nil, prov.TraceProvider, prov.MetricProvider)
	if err != nil {
		return nil, err
	}
	return service.NewAsyncLogBrtWithTelemetry(tp, level, sinks...), nil
}

// ===== Async Backpressure (Facade helpers) =====

// AsyncBackpressurePolicy re-exports service policy type for easy use from facade.
type AsyncBackpressurePolicy = service.AsyncBackpressurePolicy

// Backpressure policy constants.
const (
	DropLatest = service.DropLatest
	DropOldest = service.DropOldest
	BlockShort = service.BlockShort
)

// SetBackpressurePolicy configures the behavior when the async queue is full.
// Use with AsyncLogBrt created by NewAsyncLogBrt or NewAsyncWithOTel.
func SetBackpressurePolicy(al *AsyncLogBrt, policy AsyncBackpressurePolicy, blockFor time.Duration) {
	if al != nil {
		al.SetBackpressurePolicy(policy, blockFor)
	}
}

// NewOTelWithService returns an OTEL logger and provider, and sets the
// "service" field on the logger for consistent metric labels.
func NewOTelWithService(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*OTelLogBrt, *outboundotel.Provider, error) {
	ot, prov, err := outboundotel.NewOTel(serviceName, version, environment, endpoint, level, sinks...)
	if err != nil {
		return nil, nil, err
	}
	if ot != nil {
		if l, ok := ot.F("service", serviceName).(*service.OTelLogBrt); ok {
			ot = l
		}
	}
	return ot, prov, nil
}

// NewAsyncLogBrt creates an async logger with the given sinks.
func NewAsyncLogBrt(level Level, sinks ...Sink) *AsyncLogBrt {
    return service.NewAsyncLogBrt(level, sinks...)
}
// NewAsyncCfg creates an async logger with custom batch/workers/timeout.
func NewAsyncCfg(batchSize, workers int, timeout time.Duration, level Level, sinks ...Sink) *AsyncLogBrt {
    return service.NewAsyncLogBrtCfg(batchSize, workers, timeout, level, sinks...)
}

// NewSecurityLogBrt creates an OTEL logger with security features enabled
// (provider constructed by the outbound adapter).
func NewSecurityLogBrt(serviceName, version, environment, endpoint string, level Level, sinks ...Sink) (*SecurityLogBrt, error) {
	ot, err := outboundotel.NewOTelLogBrt(serviceName, version, environment, endpoint, level, sinks...)
	if err != nil {
		return nil, err
	}
	return secsvc.NewSecurityLogBrtWithOTel(ot)
}

// NewSecurityLogBrtWithOTel composes security features over an existing OTEL logger.
func NewSecurityLogBrtWithOTel(ot *OTelLogBrt) (*SecurityLogBrt, error) {
	return secsvc.NewSecurityLogBrtWithOTel(ot)
}

// Sinks: prefer constructing sinks via the shared factory/adapters.
var sinkFactory = sfactory.New()

// NewFastStdoutSink returns a fast JSON-to-stdout sink (for development).
func NewFastStdoutSink() Sink { return sinkFactory.FastStdout() }

// NewOptimalFileSink returns a file sink with sensible defaults.
func NewOptimalFileSink() Sink { return sinkFactory.File() }

// NewJSONSink returns a basic JSON sink implementation.
func NewJSONSink() Sink { return sinkFactory.JSON() }

// NewBufferedSink returns a buffered sink wrapper.
func NewBufferedSink() Sink { return sinkFactory.Buffered() }

// NewBufferedSinkWith returns a buffered sink with custom size/timeout.
func NewBufferedSinkWith(size int, timeout time.Duration) Sink {
    return sinkFactory.BufferedWith(size, timeout)
}
// NewBufferedWrap wraps an inner sink with a buffer.
func NewBufferedWrap(inner Sink, size int, timeout time.Duration) Sink {
    return sinkFactory.BufferedWrap(inner, size, timeout)
}

// Advanced sinks
// func NewOTLPSink(endpoint string) Sink { return core.NewOTLPSink(endpoint) }
func NewLokiPushSink(endpoint string, labels map[string]string) Sink {
	return sinkFactory.Loki(endpoint, labels)
}
func NewZerologSink() Sink             { return sinkFactory.Zerolog() }
func NewConsoleSink(enabled bool) Sink { return sinkFactory.Toggle(sinkFactory.FastStdout(), enabled) }
func NewLumberjackSink() Sink          { return sinkFactory.Lumberjack() }
func NewClickHouseSink() Sink          { return sinkFactory.ClickHouse() }

// Alerts
func NewSlackSink(webhook string) Sink { return alertsink.NewSlackSink(webhook) }
func NewTelegramSink(botToken, chatID string) Sink {
	return alertsink.NewTelegramSink(botToken, chatID)
}
func NewOpsgenieSink(apiKey string) Sink { return alertsink.NewOpsgenieSink(apiKey) }

// ===== UNIFIED SMART FACTORY =====

// New creates a logger using high-level options (no env vars required).
// Example:
//
//	log := logtrc.New(logtrc.SrvName("svc"), logtrc.Masking(true))
func New(opts ...ConfigOption) LogBrt {
	cfg := cfgopts.NewSmartConfig(opts...)
	if cfg.HasOTEL() {
		if ot, err := outboundotel.NewOTelLogBrt(cfg.GetServiceName(), cfg.GetVersion(), cfg.GetEnvironment(), cfg.GetOTELEndpoint(), cfg.GetLogLevel()); err == nil {
			return ot
		}
	}
	if cfg.HasAsync() {
		return service.NewAsyncLogBrt(cfg.GetLogLevel())
	}
	return service.NewUnifiedLogBrt(cfg.GetLogLevel())
}

// NewDefault creates a logger with sensible defaults (stdout JSON).
func NewDefault() LogBrt { return New() }

// ===== OPTIONS (Replace Environment Variables) =====

// Public option helpers for clean configuration.
var (
	SrvName    = cfgopts.SrvName
	Version    = cfgopts.Version
	Env        = cfgopts.Env
	OTel       = cfgopts.OTel
	Prometheus = cfgopts.Prometheus
	Loki       = cfgopts.Loki
	Promtail   = cfgopts.Promtail
	Masking    = cfgopts.Masking
	Async      = cfgopts.Async
	LogLevel   = cfgopts.LogLevel
)

// Response options factory
var (
    Opts = cfgopts.NewResponseOpts()
)

// ConfigOption configures the logBrt and integrations.
type ConfigOption = cfgopts.ConfigOption

// ===== LOGTRC INTEGRATION =====

// GetLogTrcFrmGin gets LogTrc from Gin context with options.
// It auto-binds TraceID/SpanID and connects to the observability stack.
func GetLogTrcFrmGin(c *gin.Context, operation string, o ...ConfigOption) *LogTrc {
    // attach operation as a field for convenience
    base := GetLog(c).F("operation", operation)
    return &LogTrc{log: base, gin: c}
}

// ===== BACKWARD COMPATIBILITY =====

// ===== GIN INTEGRATION (SIMPLIFIED) =====

// Middleware creates Gin middleware that propagates context and attaches logBrt.
func Middleware(serviceName string) gin.HandlerFunc {
    base := New().F("service", serviceName)
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

// GetOTelLog extracts OTEL-aware logBrt from Gin context (fallback to GetLog if not present)
func GetOTelLog(c *gin.Context) LogBrt {
	if logBrt, exists := c.Get("otel_log"); exists {
		if log, ok := logBrt.(LogBrt); ok {
			return log
		}
	}
	return GetLog(c)
}

// OTelMiddleware creates Gin middleware with full OTEL integration.
// Fallback to basic Middleware if inbound adapter is not available.
func OTelMiddleware(_ *OTelLogBrt) gin.HandlerFunc {
	return Middleware("otel")
}

// ===== CORRELATION UTILITIES =====

// Fs extracts and masks fields in one line (replaces ExtractFields)
// Usage: ltrace.Fs(user).Prt("User created") - extracts and masks automatically
func Fs(data interface{}) map[string]interface{} {
	extractor := &cfgopts.ExtractOptions{}
	return extractor.Fields(data)
}

// ===== HELPER FUNCTIONS =====

// (no helpers currently)

// (end facade)

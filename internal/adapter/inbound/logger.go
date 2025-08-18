package inbound

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	adapout "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
	portsout "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/outbound"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLoggerAdapter adapts zap logger to our Logger interface
type ZapLoggerAdapter struct {
	zapLogger     *zap.Logger
	fields        map[string]interface{}
	level         domain.Level
	filter        portsout.Filter
	sampler       portsout.Sampler
	formatter     portsout.Formatter
	sinks         []portsout.Sink
	loggedCount   int64
	filteredCount int64
	mu            sync.RWMutex
}

// Option configures the logger
type Option func(*ZapLoggerAdapter) error

// WithLevel sets the log level
func WithLevel(level domain.Level) Option {
	return func(l *ZapLoggerAdapter) error { l.level = level; return nil }
}

// WithSinks sets the sinks
func WithSinks(sinks ...portsout.Sink) Option {
	return func(l *ZapLoggerAdapter) error { l.sinks = sinks; return nil }
}

// NewZapLoggerAdapter creates a new zap logger adapter
func NewZapLoggerAdapter(opts ...Option) (inbound.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapLogger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create zap logger: %w", err)
	}
	adapter := &ZapLoggerAdapter{zapLogger: zapLogger, fields: make(map[string]interface{}), level: domain.InfoLevel, sinks: make([]portsout.Sink, 0)}
	for _, opt := range opts {
		if err := opt(adapter); err != nil {
			return nil, err
		}
	}
	if len(adapter.sinks) == 0 {
		adapter.sinks = append(adapter.sinks, adapout.NewStdoutSink(adapout.NewJSONFormatter()))
	}
	return adapter, nil
}

func (l *ZapLoggerAdapter) clone() *ZapLoggerAdapter {
	l.mu.RLock()
	defer l.mu.RUnlock()
	newFields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	return &ZapLoggerAdapter{zapLogger: l.zapLogger, fields: newFields, level: l.level, filter: l.filter, sampler: l.sampler, formatter: l.formatter, sinks: l.sinks, loggedCount: l.loggedCount, filteredCount: l.filteredCount}
}

// Ctx returns logger with context
func (l *ZapLoggerAdapter) Ctx(ctx context.Context) inbound.Logger {
	if ctx == nil {
		return l
	}
	newLogger := l.clone()
	if traceID := extractTraceIDFromContext(ctx); traceID != "" {
		newLogger.fields["trace_id"] = traceID
	}
	if spanID := extractSpanIDFromContext(ctx); spanID != "" {
		newLogger.fields["span_id"] = spanID
	}
	return newLogger
}

func (l *ZapLoggerAdapter) F(key string, value interface{}) inbound.Logger {
	if key == "" {
		return l
	}
	nl := l.clone()
	nl.fields[key] = value
	return nl
}
func (l *ZapLoggerAdapter) Fs(fields map[string]interface{}) inbound.Logger {
	if len(fields) == 0 {
		return l
	}
	nl := l.clone()
	for k, v := range fields {
		nl.fields[k] = v
	}
	return nl
}
func (l *ZapLoggerAdapter) Err(err error) inbound.Logger {
	if err == nil {
		return l
	}
	return l.F("error", err.Error())
}
func (l *ZapLoggerAdapter) TID(traceID string) inbound.Logger    { return l.F("trace_id", traceID) }
func (l *ZapLoggerAdapter) SID(spanID string) inbound.Logger     { return l.F("span_id", spanID) }
func (l *ZapLoggerAdapter) UID(userID string) inbound.Logger     { return l.F("user_id", userID) }
func (l *ZapLoggerAdapter) RID(requestID string) inbound.Logger  { return l.F("request_id", requestID) }
func (l *ZapLoggerAdapter) IP(ip string) inbound.Logger          { return l.F("client_ip", ip) }
func (l *ZapLoggerAdapter) Sess(sessionID string) inbound.Logger { return l.F("session_id", sessionID) }
func (l *ZapLoggerAdapter) Tenant(tenant string) inbound.Logger  { return l.F("tenant_id", tenant) }
func (l *ZapLoggerAdapter) Mod(module string) inbound.Logger     { return l.F("module", module) }

func (l *ZapLoggerAdapter) log(level domain.Level, msg string, fields ...domain.Field) {
	if level < l.level {
		atomic.AddInt64(&l.filteredCount, 1)
		return
	}
	entry := &domain.LogEntry{Level: level, Message: msg, Fields: make(map[string]interface{}), Timestamp: time.Now()}
	l.mu.RLock()
	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	l.mu.RUnlock()
	for _, f := range fields {
		entry.Fields[f.Key] = f.Value
	}
	if l.filter != nil {
		entry = l.filter.Apply(entry)
		if entry == nil {
			atomic.AddInt64(&l.filteredCount, 1)
			return
		}
	}
	if l.sampler != nil && !l.sampler.Sample(entry) {
		atomic.AddInt64(&l.filteredCount, 1)
		return
	}
	for _, sink := range l.sinks {
		if err := sink.Write(entry); err != nil {
			fmt.Printf("Failed to write to sink %s: %v\n", sink.Name(), err)
		}
	}
	atomic.AddInt64(&l.loggedCount, 1)
}

func (l *ZapLoggerAdapter) Debug(msg string, fields ...domain.Field) {
	l.log(domain.DebugLevel, msg, fields...)
}
func (l *ZapLoggerAdapter) Info(msg string, fields ...domain.Field) {
	l.log(domain.InfoLevel, msg, fields...)
}
func (l *ZapLoggerAdapter) Warn(msg string, fields ...domain.Field) {
	l.log(domain.WarnLevel, msg, fields...)
}
func (l *ZapLoggerAdapter) Error(msg string, fields ...domain.Field) {
	l.log(domain.ErrorLevel, msg, fields...)
}
func (l *ZapLoggerAdapter) Fatal(msg string, fields ...domain.Field) {
	l.log(domain.FatalLevel, msg, fields...)
}
func (l *ZapLoggerAdapter) Level(level domain.Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}
func (l *ZapLoggerAdapter) GetLevel() domain.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}
func (l *ZapLoggerAdapter) Logged() int64   { return atomic.LoadInt64(&l.loggedCount) }
func (l *ZapLoggerAdapter) Filtered() int64 { return atomic.LoadInt64(&l.filteredCount) }

// Helper functions
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

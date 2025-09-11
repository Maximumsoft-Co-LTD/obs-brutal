package base

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"obs-brutal/internal/util"
)

// LogBrt defines fluent logbrut behavior
type LogBrt interface {
	F(key string, value interface{}) LogBrt
	Fs(fields map[string]interface{}) LogBrt
	Ctx(ctx context.Context) LogBrt
	TraceID(id string) LogBrt
	UserID(id string) LogBrt
	RequestID(id string) LogBrt
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	WithError(err error) LogBrt
	LogCount() int64
}

type UnifiedLogBrt struct {
	level    domain.Level
	sinks    []port.Sink
	fields   map[string]interface{}
	logCount *atomic.Int64
	isClone  bool
}

type defaultStdoutSink struct{ port.SinkBase }

func (s *defaultStdoutSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	return util.WriteJSONToWriter(os.Stdout, entry)
}
func (s *defaultStdoutSink) Name() string { return "stdout" }

func NewUnifiedLogBrt(level domain.Level, sinks ...port.Sink) *UnifiedLogBrt {
	if len(sinks) == 0 {
		sinks = []port.Sink{&defaultStdoutSink{}}
	}
	return &UnifiedLogBrt{level: level, sinks: sinks, fields: make(map[string]interface{}, 8), logCount: &atomic.Int64{}}
}

func (l *UnifiedLogBrt) F(key string, value interface{}) LogBrt {
	c := l.clone()
	c.fields[key] = value
	return c
}
func (l *UnifiedLogBrt) Fs(fields map[string]interface{}) LogBrt {
	if len(fields) == 0 {
		return l
	}
	c := l.clone()
	for k, v := range fields {
		c.fields[k] = v
	}
	return c
}
func (l *UnifiedLogBrt) Ctx(ctx context.Context) LogBrt {
	if ctx == nil {
		return l
	}
	c := l.clone()
	c.fields["context"] = ctx
	if v := util.GetTraceID(ctx); v != "" {
		c.fields["trace_id"] = v
	}
	if v := util.GetUserID(ctx); v != "" {
		c.fields["user_id"] = v
	}
	if v := util.GetRequestID(ctx); v != "" {
		c.fields["request_id"] = v
	}
	return c
}
func (l *UnifiedLogBrt) TraceID(id string) LogBrt   { return l.F("trace_id", id) }
func (l *UnifiedLogBrt) UserID(id string) LogBrt    { return l.F("user_id", id) }
func (l *UnifiedLogBrt) RequestID(id string) LogBrt { return l.F("request_id", id) }
func (l *UnifiedLogBrt) WithError(err error) LogBrt {
	if err == nil {
		return l
	}
	return l.F("error", err.Error())
}

func (l *UnifiedLogBrt) Debug(msg string) { l.log(domain.DebugLevel, msg) }
func (l *UnifiedLogBrt) Info(msg string)  { l.log(domain.InfoLevel, msg) }
func (l *UnifiedLogBrt) Warn(msg string)  { l.log(domain.WarnLevel, msg) }
func (l *UnifiedLogBrt) Error(msg string) { l.log(domain.ErrorLevel, msg) }
func (l *UnifiedLogBrt) Fatal(msg string) { l.log(domain.FatalLevel, msg) }
func (l *UnifiedLogBrt) Debugf(format string, args ...interface{}) {
	l.log(domain.DebugLevel, appendf(format, args...))
}
func (l *UnifiedLogBrt) Infof(format string, args ...interface{}) {
	l.log(domain.InfoLevel, appendf(format, args...))
}
func (l *UnifiedLogBrt) Warnf(format string, args ...interface{}) {
	l.log(domain.WarnLevel, appendf(format, args...))
}
func (l *UnifiedLogBrt) Errorf(format string, args ...interface{}) {
	l.log(domain.ErrorLevel, appendf(format, args...))
}
func (l *UnifiedLogBrt) Fatalf(format string, args ...interface{}) {
	l.log(domain.FatalLevel, appendf(format, args...))
}

func (l *UnifiedLogBrt) LogCount() int64 {
	if l.logCount == nil {
		return 0
	}
	return l.logCount.Load()
}

func (l *UnifiedLogBrt) clone() *UnifiedLogBrt {
	c := &UnifiedLogBrt{level: l.level, sinks: l.sinks, fields: make(map[string]interface{}, len(l.fields)+2), isClone: true}
	for k, v := range l.fields {
		c.fields[k] = v
	}
	c.logCount = l.logCount
	return c
}
func (l *UnifiedLogBrt) log(level domain.Level, msg string) {
	if level < l.level {
		return
	}
	if len(l.fields) == 0 { // no-fields fast path (avoid map alloc)
		entry := &domain.LogEntry{Level: level, Msg: msg, Timestamp: time.Now()}
		for _, s := range l.sinks {
			if s != nil {
				_ = s.Write(entry)
			}
		}
		if l.logCount != nil {
			l.logCount.Add(1)
		}
		return
	}
	entry := newLogEntry(level, msg, l.fields)
	for _, s := range l.sinks {
		if s != nil {
			_ = s.Write(entry)
		}
	}
	if l.logCount != nil {
		l.logCount.Add(1)
	}
}

func newLogEntry(level domain.Level, msg string, baseFields map[string]interface{}) *domain.LogEntry {
	if len(baseFields) == 0 {
		return &domain.LogEntry{Level: level, Msg: msg, Timestamp: time.Now()}
	}
	entry := &domain.LogEntry{Level: level, Msg: msg, Timestamp: time.Now(), F: make(map[string]interface{}, len(baseFields))}
	for k, v := range baseFields {
		entry.F[k] = v
	}
	Promote(entry)
	return entry
}

// Helpers for cross‑package usage
func (l *UnifiedLogBrt) FieldsCopy() map[string]interface{} {
	out := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		out[k] = v
	}
	return out
}
func (l *UnifiedLogBrt) WithOnlyFields(m map[string]interface{}) *UnifiedLogBrt {
	c := l.clone()
	c.fields = make(map[string]interface{}, len(m))
	for k, v := range m {
		c.fields[k] = v
	}
	return c
}
func (l *UnifiedLogBrt) Level() domain.Level { return l.level }
func (l *UnifiedLogBrt) IncCount() {
	if l.logCount != nil {
		l.logCount.Add(1)
	}
}
func (l *UnifiedLogBrt) WriteEntry(entry *domain.LogEntry) {
	for _, s := range l.sinks {
		if s != nil {
			_ = s.Write(entry)
		}
	}
}

// extractFromContext was removed; use util.GetTraceID/GetUserID/GetRequestID

// appendf builds formatted string with fewer temporaries on recent Go versions
func appendf(format string, args ...interface{}) string {
	// Go 1.22+ supports fmt.Appendf; fallback to Sprintf otherwise
	// Using Appendf reduces intermediate allocations
	b := make([]byte, 0, len(format)+32)
	b = fmt.Appendf(b, format, args...)
	return string(b)
}

// Promote moves well-known IDs from F into top-level fields for JSON writers.
// It avoids duplicate output and ensures util.WriteJSONToWriter includes IDs.
func Promote(e *domain.LogEntry) {
	if e == nil || e.F == nil {
		return
	}
	if e.TraceID == "" {
		if v, ok := e.F["trace_id"].(string); ok {
			e.TraceID = v
			delete(e.F, "trace_id")
		}
	}
	if e.SpanID == "" {
		if v, ok := e.F["span_id"].(string); ok {
			e.SpanID = v
			delete(e.F, "span_id")
		}
	}
	if e.RequestID == "" {
		if v, ok := e.F["request_id"].(string); ok {
			e.RequestID = v
			delete(e.F, "request_id")
		}
	}
	if e.UserID == "" {
		if v, ok := e.F["user_id"].(string); ok {
			e.UserID = v
			delete(e.F, "user_id")
		}
	}
	if e.Mod == "" {
		if v, ok := e.F["module"].(string); ok {
			e.Mod = v
			delete(e.F, "module")
		}
	}
	if e.TenantID == "" {
		if v, ok := e.F["tenant_id"].(string); ok {
			e.TenantID = v
			delete(e.F, "tenant_id")
		}
	}

	delete(e.F, "context")
}

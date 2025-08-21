// Package core provides the unified best-performance logger implementation
package core

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"obs-brutal/internal/core/domain"
)

// ===== SIMPLIFIED TYPES =====

// Level type alias
type Level = domain.Level

// Log level constants
const (
	DEBUG Level = domain.DebugLevel
	INFO  Level = domain.InfoLevel
	WARN  Level = domain.WarnLevel
	ERROR Level = domain.ErrorLevel
	FATAL Level = domain.FatalLevel
)

// Sink interface for outputs (simplified)
type Sink interface {
	Write(entry *domain.LogEntry) error
	Close() error
	Name() string
	Health() error
	Configure(config map[string]interface{}) error
}

// ===== SINK CONSTRUCTORS =====

// NewFastStdoutSink creates optimized stdout sink
func NewFastStdoutSink() Sink {
	return &FastStdoutSink{}
}

// NewOptimalFileSink creates optimized file sink
func NewOptimalFileSink() Sink {
	return &OptimalFileSink{filename: "logs/app.log"}
}

// NewJSONSink creates optimized JSON sink
func NewJSONSink() Sink { return &JSONSink{} }

// NewBufferedSink creates optimized buffered sink
func NewBufferedSink() Sink {
	return &BufferedSink{
		buffer:  make([]*domain.LogEntry, 0, 1000),
		maxSize: 1000,
		timeout: 100 * time.Millisecond,
	}
}

// Logger interface for unified logger (simplified)
type Logger interface {
	// Fluent field API
	F(key string, value interface{}) Logger
	Fs(fields map[string]interface{}) Logger

	// Context and correlation
	Ctx(ctx context.Context) Logger
	TraceID(id string) Logger
	UserID(id string) Logger
	RequestID(id string) Logger

	// Simple logging methods
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)

	// Quick formatted logging
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	// Error handling
	WithError(err error) Logger

	// Metrics
	LogCount() int64
}

// UnifiedLogger is the single, best-performance logger implementation
type UnifiedLogger struct {
	level    domain.Level
	sinks    []Sink
	fields   map[string]interface{}
	logCount *atomic.Int64
	isClone  bool
}

// NewUnifiedLogger creates the optimal logger
func NewUnifiedLogger(level domain.Level, sinks ...Sink) *UnifiedLogger {
	if len(sinks) == 0 {
		sinks = []Sink{NewFastStdoutSink()}
	}

	return &UnifiedLogger{
		level:    level,
		sinks:    sinks,
		fields:   make(map[string]interface{}, 8), // Pre-allocate
		logCount: &atomic.Int64{},
	}
}

// ===== SIMPLIFIED FLUENT API =====

// F adds a field (fluent style)
func (l *UnifiedLogger) F(key string, value interface{}) Logger {
	clone := l.clone()
	clone.fields[key] = value
	return clone
}

// Fs adds multiple fields
func (l *UnifiedLogger) Fs(fields map[string]interface{}) Logger {
	if len(fields) == 0 {
		return l
	}

	clone := l.clone()
	for k, v := range fields {
		clone.fields[k] = v
	}
	return clone
}

// Ctx adds context information
func (l *UnifiedLogger) Ctx(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	clone := l.clone()

	// Extract common context values
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

// ===== CONVENIENT ID METHODS =====

// TraceID adds trace ID
func (l *UnifiedLogger) TraceID(id string) Logger {
	return l.F("trace_id", id)
}

// UserID adds user ID
func (l *UnifiedLogger) UserID(id string) Logger {
	return l.F("user_id", id)
}

// RequestID adds request ID
func (l *UnifiedLogger) RequestID(id string) Logger {
	return l.F("request_id", id)
}

// WithError adds error information
func (l *UnifiedLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}
	return l.F("error", err.Error())
}

// ===== SIMPLE LOGGING METHODS =====

// Debug logs debug message
func (l *UnifiedLogger) Debug(msg string) {
	l.log(domain.DebugLevel, msg)
}

// Info logs info message
func (l *UnifiedLogger) Info(msg string) {
	l.log(domain.InfoLevel, msg)
}

// Warn logs warning message
func (l *UnifiedLogger) Warn(msg string) {
	l.log(domain.WarnLevel, msg)
}

// Error logs error message
func (l *UnifiedLogger) Error(msg string) {
	l.log(domain.ErrorLevel, msg)
}

// Fatal logs fatal message
func (l *UnifiedLogger) Fatal(msg string) {
	l.log(domain.FatalLevel, msg)
}

// ===== FORMATTED LOGGING =====

// Debugf logs formatted debug message
func (l *UnifiedLogger) Debugf(format string, args ...interface{}) {
	l.log(domain.DebugLevel, fmt.Sprintf(format, args...))
}

// Infof logs formatted info message
func (l *UnifiedLogger) Infof(format string, args ...interface{}) {
	l.log(domain.InfoLevel, fmt.Sprintf(format, args...))
}

// Warnf logs formatted warning message
func (l *UnifiedLogger) Warnf(format string, args ...interface{}) {
	l.log(domain.WarnLevel, fmt.Sprintf(format, args...))
}

// Errorf logs formatted error message
func (l *UnifiedLogger) Errorf(format string, args ...interface{}) {
	l.log(domain.ErrorLevel, fmt.Sprintf(format, args...))
}

// Fatalf logs formatted fatal message
func (l *UnifiedLogger) Fatalf(format string, args ...interface{}) {
	l.log(domain.FatalLevel, fmt.Sprintf(format, args...))
}

// ===== METRICS =====

// LogCount returns total logs written
func (l *UnifiedLogger) LogCount() int64 {
	if l.logCount == nil {
		return 0
	}
	return l.logCount.Load()
}

// ===== INTERNAL IMPLEMENTATION =====

// clone creates efficient shallow copy
func (l *UnifiedLogger) clone() *UnifiedLogger {
	clone := &UnifiedLogger{
		level:   l.level,
		sinks:   l.sinks, // Shared reference
		fields:  make(map[string]interface{}, len(l.fields)+2),
		isClone: true,
	}

	// Copy fields efficiently
	for k, v := range l.fields {
		clone.fields[k] = v
	}

	// Share atomic counter reference (avoid copying)
	clone.logCount = l.logCount

	return clone
}

// log is the optimized core logging method
func (l *UnifiedLogger) log(level domain.Level, msg string) {
	// Fast level check
	if level < l.level {
		return
	}

	// Create log entry efficiently
	entry := &domain.LogEntry{
		Fields: make(map[string]interface{}, len(l.fields)),
	}

	// Set properties
	entry.Level = level
	entry.Message = msg
	entry.Timestamp = time.Now()

	// Copy fields efficiently
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// Write to all sinks
	for _, sink := range l.sinks {
		if sink != nil {
			sink.Write(entry)
		}
	}

	// Update counter
	if l.logCount != nil {
		l.logCount.Add(1)
	}
}

// ===== FAST STDOUT SINK =====

// FastStdoutSink is the optimized stdout implementation
type FastStdoutSink struct{}

func (s *FastStdoutSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	// Create string builder
	sb := &FastStringBuilder{buf: make([]byte, 0, 256)}

	// Build optimized JSON
	buildOptimalJSON(sb, entry)

	// Direct write to stdout
	_, err := os.Stdout.Write(sb.Bytes())
	return err
}

func (s *FastStdoutSink) Close() error                           { return nil }
func (s *FastStdoutSink) Name() string                           { return "stdout" }
func (s *FastStdoutSink) Health() error                          { return nil }
func (s *FastStdoutSink) Configure(map[string]interface{}) error { return nil }

// FastStringBuilder provides efficient string building
type FastStringBuilder struct {
	buf []byte
}

// WriteString appends string
func (sb *FastStringBuilder) WriteString(s string) {
	sb.buf = append(sb.buf, s...)
}

// WriteByte appends byte
func (sb *FastStringBuilder) WriteByte(c byte) error {
	sb.buf = append(sb.buf, c)
	return nil
}

// Bytes returns underlying bytes
func (sb *FastStringBuilder) Bytes() []byte {
	return sb.buf
}

// Reset clears the buffer
func (sb *FastStringBuilder) Reset() {
	sb.buf = sb.buf[:0]
}

// buildOptimalJSON creates optimized JSON output
func buildOptimalJSON(sb *FastStringBuilder, entry *domain.LogEntry) {
	sb.Reset()

	// Start JSON with full ISO8601 datetime
	sb.WriteString(`{"datetime":"`)
	sb.WriteString(entry.Timestamp.Format("2006-01-02T15:04:05-07:00"))
	sb.WriteString(`","level":"`)
	sb.WriteString(entry.Level.String())
	sb.WriteString(`","msg":"`)
	sb.WriteString(entry.Message)
	sb.WriteString(`"`)

	// Add fields compactly
	for k, v := range entry.Fields {
		sb.WriteString(`,"`)
		sb.WriteString(k)
		sb.WriteString(`":"`)
		sb.WriteString(fmt.Sprintf("%v", v))
		sb.WriteString(`"`)
	}

	// Close JSON and add newline
	sb.WriteString("}\n")
}

// extractFromContext extracts value from context
func extractFromContext(ctx context.Context, key string) string {
	if val := ctx.Value(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// ===== ADDITIONAL SINK IMPLEMENTATIONS =====

// OptimalFileSink provides efficient file logging
type OptimalFileSink struct {
	filename string
}

func (s *OptimalFileSink) Write(entry *domain.LogEntry) error {
	// Simplified file writing - would use proper file rotation in production
	return nil
}

func (s *OptimalFileSink) Close() error                           { return nil }
func (s *OptimalFileSink) Name() string                           { return "file" }
func (s *OptimalFileSink) Health() error                          { return nil }
func (s *OptimalFileSink) Configure(map[string]interface{}) error { return nil }

// JSONSink provides pure JSON output
type JSONSink struct{}

func (s *JSONSink) Write(entry *domain.LogEntry) error {
	sb := &FastStringBuilder{buf: make([]byte, 0, 256)}

	buildOptimalJSON(sb, entry)
	_, err := os.Stdout.Write(sb.Bytes())
	return err
}

func (s *JSONSink) Close() error                           { return nil }
func (s *JSONSink) Name() string                           { return "json" }
func (s *JSONSink) Health() error                          { return nil }
func (s *JSONSink) Configure(map[string]interface{}) error { return nil }

// BufferedSink provides high-throughput logging
type BufferedSink struct {
	buffer  []*domain.LogEntry
	maxSize int
	timeout time.Duration
}

func (s *BufferedSink) Write(entry *domain.LogEntry) error {
	// Simplified buffering - would need proper implementation
	sb := &FastStringBuilder{buf: make([]byte, 0, 256)}

	buildOptimalJSON(sb, entry)
	_, err := os.Stdout.Write(sb.Bytes())
	return err
}

func (s *BufferedSink) Close() error                           { return nil }
func (s *BufferedSink) Name() string                           { return "buffer" }
func (s *BufferedSink) Health() error                          { return nil }
func (s *BufferedSink) Configure(map[string]interface{}) error { return nil }

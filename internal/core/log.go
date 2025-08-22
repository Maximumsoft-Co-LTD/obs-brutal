// Package core provides the unified best-performance logBrt implementation
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
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

// SinkBase provides no-op implementations for optional Sink methods.
// Embed in sinks to avoid repeating trivial methods.
type SinkBase struct{}

func (SinkBase) Close() error                           { return nil }
func (SinkBase) Health() error                          { return nil }
func (SinkBase) Configure(map[string]interface{}) error { return nil }

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
func NewBufferedSink() Sink { return NewBufferedSinkWith(1000, 100*time.Millisecond) }

// NewBufferedSinkWith creates optimized buffered sink with custom size and timeout
func NewBufferedSinkWith(size int, timeout time.Duration) Sink {
	s := &BufferedSink{
		buffer:  make([]*domain.LogEntry, 0, size),
		maxSize: size,
		timeout: timeout,
	}
	s.startFlusher()
	return s
}

// LogBrt interface for unified logBrt (simplified)
type LogBrt interface {
	// Fluent field API
	F(key string, value interface{}) LogBrt
	Fs(fields map[string]interface{}) LogBrt

	// Context and correlation
	Ctx(ctx context.Context) LogBrt
	TraceID(id string) LogBrt
	UserID(id string) LogBrt
	RequestID(id string) LogBrt

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
	WithError(err error) LogBrt

	// Metrics
	LogCount() int64
}

// UnifiedLogBrt is the single, best-performance logBrt implementation
type UnifiedLogBrt struct {
	level    domain.Level
	sinks    []Sink
	fields   map[string]interface{}
	logCount *atomic.Int64
	isClone  bool
}

// NewUnifiedLogBrt creates the optimal logBrt
func NewUnifiedLogBrt(level domain.Level, sinks ...Sink) *UnifiedLogBrt {
	if len(sinks) == 0 {
		sinks = []Sink{NewFastStdoutSink()}
	}

	return &UnifiedLogBrt{
		level:    level,
		sinks:    sinks,
		fields:   make(map[string]interface{}, 8), // Pre-allocate
		logCount: &atomic.Int64{},
	}
}

// ===== SIMPLIFIED FLUENT API =====

// F adds a field (fluent style)
func (l *UnifiedLogBrt) F(key string, value interface{}) LogBrt {
	clone := l.clone()
	clone.fields[key] = value
	return clone
}

// Fs adds multiple fields
func (l *UnifiedLogBrt) Fs(fields map[string]interface{}) LogBrt {
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
func (l *UnifiedLogBrt) Ctx(ctx context.Context) LogBrt {
	if ctx == nil {
		return l
	}

	clone := l.clone()

	// Preserve original context for further propagation (OTEL/middleware)
	clone.fields["context"] = ctx

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
func (l *UnifiedLogBrt) TraceID(id string) LogBrt {
	return l.F("trace_id", id)
}

// UserID adds user ID
func (l *UnifiedLogBrt) UserID(id string) LogBrt {
	return l.F("user_id", id)
}

// RequestID adds request ID
func (l *UnifiedLogBrt) RequestID(id string) LogBrt {
	return l.F("request_id", id)
}

// WithError adds error information
func (l *UnifiedLogBrt) WithError(err error) LogBrt {
	if err == nil {
		return l
	}
	return l.F("error", err.Error())
}

// ===== SIMPLE LOGGING METHODS =====

// Debug logs debug message
func (l *UnifiedLogBrt) Debug(msg string) {
	l.log(domain.DebugLevel, msg)
}

// Info logs info message
func (l *UnifiedLogBrt) Info(msg string) {
	l.log(domain.InfoLevel, msg)
}

// Warn logs warning message
func (l *UnifiedLogBrt) Warn(msg string) {
	l.log(domain.WarnLevel, msg)
}

// Error logs error message
func (l *UnifiedLogBrt) Error(msg string) {
	l.log(domain.ErrorLevel, msg)
}

// Fatal logs fatal message
func (l *UnifiedLogBrt) Fatal(msg string) {
	l.log(domain.FatalLevel, msg)
}

// ===== FORMATTED LOGGING =====

// Debugf logs formatted debug message
func (l *UnifiedLogBrt) Debugf(format string, args ...interface{}) {
	l.log(domain.DebugLevel, fmt.Sprintf(format, args...))
}

// Infof logs formatted info message
func (l *UnifiedLogBrt) Infof(format string, args ...interface{}) {
	l.log(domain.InfoLevel, fmt.Sprintf(format, args...))
}

// Warnf logs formatted warning message
func (l *UnifiedLogBrt) Warnf(format string, args ...interface{}) {
	l.log(domain.WarnLevel, fmt.Sprintf(format, args...))
}

// Errorf logs formatted error message
func (l *UnifiedLogBrt) Errorf(format string, args ...interface{}) {
	l.log(domain.ErrorLevel, fmt.Sprintf(format, args...))
}

// Fatalf logs formatted fatal message
func (l *UnifiedLogBrt) Fatalf(format string, args ...interface{}) {
	l.log(domain.FatalLevel, fmt.Sprintf(format, args...))
}

// ===== METRICS =====

// LogCount returns total logs written
func (l *UnifiedLogBrt) LogCount() int64 {
	if l.logCount == nil {
		return 0
	}
	return l.logCount.Load()
}

// ===== INTERNAL IMPLEMENTATION =====

// clone creates efficient shallow copy
func (l *UnifiedLogBrt) clone() *UnifiedLogBrt {
	clone := &UnifiedLogBrt{
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
func (l *UnifiedLogBrt) log(level domain.Level, msg string) {
	// Fast level check
	if level < l.level {
		return
	}

	// Create log entry efficiently
	entry := newLogEntry(level, msg, l.fields)

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
type FastStdoutSink struct{ SinkBase }

func (s *FastStdoutSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	return writeJSONToWriter(os.Stdout, entry)
}

func (s *FastStdoutSink) Name() string { return "stdout" }

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
	SinkBase
	filename string
	file     *os.File
	mu       sync.Mutex
}

func (s *OptimalFileSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		// Ensure directory exists
		dir := filepath.Dir(s.filename)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.OpenFile(s.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		s.file = f
	}

	return writeJSONToWriter(s.file, entry)
}

func (s *OptimalFileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		return err
	}
	return nil
}
func (s *OptimalFileSink) Name() string { return "file" }

// JSONSink provides pure JSON output
type JSONSink struct{ SinkBase }

func (s *JSONSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	return writeJSONToWriter(os.Stdout, entry)
}

func (s *JSONSink) Name() string { return "json" }

// BufferedSink provides high-throughput logging
type BufferedSink struct {
	SinkBase
	buffer    []*domain.LogEntry
	maxSize   int
	timeout   time.Duration
	lastFlush time.Time
	mu        sync.Mutex
	stopCh    chan struct{}
}

func (s *BufferedSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer = append(s.buffer, entry)

	// Flush on size
	if len(s.buffer) >= s.maxSize {
		return s.flushLocked()
	}

	// Opportunistic timeout flush
	if !s.lastFlush.IsZero() && time.Since(s.lastFlush) >= s.timeout {
		return s.flushLocked()
	}

	return nil
}

func (s *BufferedSink) Close() error {
	// signal stop
	if s.stopCh != nil {
		close(s.stopCh)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}
func (s *BufferedSink) Name() string { return "buffer" }

func (s *BufferedSink) flushLocked() error {
	for _, e := range s.buffer {
		_ = writeJSONToWriter(os.Stdout, e)
	}
	s.buffer = s.buffer[:0]
	s.lastFlush = time.Now()
	return nil
}

// startFlusher launches a background flusher if not started
func (s *BufferedSink) startFlusher() {
	s.mu.Lock()
	if s.stopCh != nil {
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	interval := s.timeout
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	s.mu.Unlock()

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-t.C:
				s.mu.Lock()
				if len(s.buffer) > 0 {
					_ = s.flushLocked()
				}
				s.mu.Unlock()
			}
		}
	}()
}

func writeJSONToWriter(w io.Writer, entry *domain.LogEntry) error {
	// Ordered JSON output: datetime, level, msg, trace_id, span_id, request_id, then other fields (sorted)
	var buf bytes.Buffer
	buf.WriteByte('{')

	writeKV := func(key string, val interface{}, isFirst *bool) error {
		if !*isFirst {
			buf.WriteByte(',')
		}
		*isFirst = false
		buf.WriteString(strconv.Quote(key))
		buf.WriteByte(':')
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}

	isFirst := true
	if err := writeKV("datetime", entry.Timestamp.Format("2006-01-02T15:04:05-07:00"), &isFirst); err != nil {
		return err
	}
	if err := writeKV("level", entry.Level.String(), &isFirst); err != nil {
		return err
	}
	if err := writeKV("msg", entry.Message, &isFirst); err != nil {
		return err
	}

	// Correlation fields
	traceID := entry.TraceID
	if traceID == "" {
		if v, ok := entry.Fields["trace_id"]; ok {
			if s, ok2 := v.(string); ok2 {
				traceID = s
			}
		}
	}
	if traceID != "" {
		if err := writeKV("trace_id", traceID, &isFirst); err != nil {
			return err
		}
	}

	spanID := entry.SpanID
	if spanID == "" {
		if v, ok := entry.Fields["span_id"]; ok {
			if s, ok2 := v.(string); ok2 {
				spanID = s
			}
		}
	}
	if spanID != "" {
		if err := writeKV("span_id", spanID, &isFirst); err != nil {
			return err
		}
	}

	requestID := entry.RequestID
	if requestID == "" {
		if v, ok := entry.Fields["request_id"]; ok {
			if s, ok2 := v.(string); ok2 {
				requestID = s
			}
		}
	}
	if requestID != "" {
		if err := writeKV("request_id", requestID, &isFirst); err != nil {
			return err
		}
	}

	// Remaining fields (skip reserved and internal context)
	reserved := map[string]struct{}{
		"datetime": {}, "level": {}, "msg": {},
		"trace_id": {}, "span_id": {}, "request_id": {},
		"context": {},
	}
	keys := make([]string, 0, len(entry.Fields))
	for k := range entry.Fields {
		if _, ok := reserved[k]; ok {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := writeKV(k, entry.Fields[k], &isFirst); err != nil {
			return err
		}
	}

	buf.WriteByte('}')
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// newLogEntry creates a log entry from level, message and base fields.
// It performs an efficient copy of fields and sets timestamp.
func newLogEntry(level domain.Level, msg string, baseFields map[string]interface{}) *domain.LogEntry {
	entry := &domain.LogEntry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}, len(baseFields)),
	}
	for k, v := range baseFields {
		entry.Fields[k] = v
	}
	return entry
}

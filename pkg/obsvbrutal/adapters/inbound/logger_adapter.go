package inbound

import (
	"context"
	"fmt"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/inbound"
	"obs-brutal/pkg/obsvbrutal/core/ports/outbound"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLoggerAdapter adapts zap logger to our Logger interface
type ZapLoggerAdapter struct {
	zapLogger     *zap.Logger
	fields        map[string]interface{}
	level         domain.Level
	filter        outbound.Filter
	sampler       outbound.Sampler
	formatter     outbound.Formatter
	sinks         []outbound.Sink
	loggedCount   int64
	filteredCount int64
	mu            sync.RWMutex
}

// Option configures the logger
type Option func(*ZapLoggerAdapter) error

// WithLevel sets the log level
func WithLevel(level domain.Level) Option {
	return func(l *ZapLoggerAdapter) error {
		l.level = level
		return nil
	}
}

// WithSinks sets the sinks
func WithSinks(sinks ...outbound.Sink) Option {
	return func(l *ZapLoggerAdapter) error {
		l.sinks = sinks
		return nil
	}
}

// NewZapLoggerAdapter creates a new zap logger adapter
func NewZapLoggerAdapter(opts ...Option) (inbound.Logger, error) {
	// Create zap config
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create zap logger
	zapLogger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create zap logger: %w", err)
	}

	adapter := &ZapLoggerAdapter{
		zapLogger: zapLogger,
		fields:    make(map[string]interface{}),
		level:     domain.InfoLevel,
		sinks:     make([]outbound.Sink, 0),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(adapter); err != nil {
			return nil, err
		}
	}

	// Ensure at least one sink
	if len(adapter.sinks) == 0 {
		adapter.sinks = append(adapter.sinks, NewStdoutSink())
	}

	return adapter, nil
}

// Clone creates a copy of the logger
func (l *ZapLoggerAdapter) clone() *ZapLoggerAdapter {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		newFields[k] = v
	}

	return &ZapLoggerAdapter{
		zapLogger:     l.zapLogger,
		fields:        newFields,
		level:         l.level,
		filter:        l.filter,
		sampler:       l.sampler,
		formatter:     l.formatter,
		sinks:         l.sinks,
		loggedCount:   l.loggedCount,
		filteredCount: l.filteredCount,
	}
}

// Ctx returns logger with context
func (l *ZapLoggerAdapter) Ctx(ctx context.Context) inbound.Logger {
	if ctx == nil {
		return l
	}

	newLogger := l.clone()

	// Extract values from context
	if traceID := extractTraceIDFromContext(ctx); traceID != "" {
		newLogger.fields["trace_id"] = traceID
	}
	if spanID := extractSpanIDFromContext(ctx); spanID != "" {
		newLogger.fields["span_id"] = spanID
	}

	return newLogger
}

// F returns logger with field
func (l *ZapLoggerAdapter) F(key string, value interface{}) inbound.Logger {
	if key == "" {
		return l
	}

	newLogger := l.clone()
	newLogger.fields[key] = value
	return newLogger
}

// Fs returns logger with fields
func (l *ZapLoggerAdapter) Fs(fields map[string]interface{}) inbound.Logger {
	if len(fields) == 0 {
		return l
	}

	newLogger := l.clone()
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// Err returns logger with error
func (l *ZapLoggerAdapter) Err(err error) inbound.Logger {
	if err == nil {
		return l
	}
	return l.F("error", err.Error())
}

// TID returns logger with trace ID
func (l *ZapLoggerAdapter) TID(traceID string) inbound.Logger {
	return l.F("trace_id", traceID)
}

// SID returns logger with span ID
func (l *ZapLoggerAdapter) SID(spanID string) inbound.Logger {
	return l.F("span_id", spanID)
}

// UID returns logger with user ID
func (l *ZapLoggerAdapter) UID(userID string) inbound.Logger {
	return l.F("user_id", userID)
}

// RID returns logger with request ID
func (l *ZapLoggerAdapter) RID(requestID string) inbound.Logger {
	return l.F("request_id", requestID)
}

// IP returns logger with client IP
func (l *ZapLoggerAdapter) IP(ip string) inbound.Logger {
	return l.F("client_ip", ip)
}

// Sess returns logger with session ID
func (l *ZapLoggerAdapter) Sess(sessionID string) inbound.Logger {
	return l.F("session_id", sessionID)
}

// Tenant returns logger with tenant
func (l *ZapLoggerAdapter) Tenant(tenant string) inbound.Logger {
	return l.F("tenant_id", tenant)
}

// Mod returns logger with module
func (l *ZapLoggerAdapter) Mod(module string) inbound.Logger {
	return l.F("module", module)
}

// Log generic log method
func (l *ZapLoggerAdapter) log(level domain.Level, msg string, fields ...domain.Field) {
	// Check level
	if level < l.level {
		atomic.AddInt64(&l.filteredCount, 1)
		return
	}

	// Create log entry
	entry := &domain.LogEntry{
		Level:     level,
		Message:   msg,
		Fields:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Copy fields
	l.mu.RLock()
	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	l.mu.RUnlock()

	// Add additional fields
	for _, f := range fields {
		entry.Fields[f.Key] = f.Value
	}

	// Apply filter
	if l.filter != nil {
		entry = l.filter.Apply(entry)
		if entry == nil {
			atomic.AddInt64(&l.filteredCount, 1)
			return
		}
	}

	// Apply sampler
	if l.sampler != nil && !l.sampler.Sample(entry) {
		atomic.AddInt64(&l.filteredCount, 1)
		return
	}

	// Write to sinks
	for _, sink := range l.sinks {
		if err := sink.Write(entry); err != nil {
			// Log error but continue
			fmt.Printf("Failed to write to sink %s: %v\n", sink.Name(), err)
		}
	}

	atomic.AddInt64(&l.loggedCount, 1)
}

// Debug logs debug message
func (l *ZapLoggerAdapter) Debug(msg string, fields ...domain.Field) {
	l.log(domain.DebugLevel, msg, fields...)
}

// Info logs info message
func (l *ZapLoggerAdapter) Info(msg string, fields ...domain.Field) {
	l.log(domain.InfoLevel, msg, fields...)
}

// Warn logs warning message
func (l *ZapLoggerAdapter) Warn(msg string, fields ...domain.Field) {
	l.log(domain.WarnLevel, msg, fields...)
}

// Error logs error message
func (l *ZapLoggerAdapter) Error(msg string, fields ...domain.Field) {
	l.log(domain.ErrorLevel, msg, fields...)
}

// Fatal logs fatal message
func (l *ZapLoggerAdapter) Fatal(msg string, fields ...domain.Field) {
	l.log(domain.FatalLevel, msg, fields...)
}

// Level sets log level
func (l *ZapLoggerAdapter) Level(level domain.Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns current log level
func (l *ZapLoggerAdapter) GetLevel() domain.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// Logged returns logged count
func (l *ZapLoggerAdapter) Logged() int64 {
	return atomic.LoadInt64(&l.loggedCount)
}

// Filtered returns filtered count
func (l *ZapLoggerAdapter) Filtered() int64 {
	return atomic.LoadInt64(&l.filteredCount)
}

// Helper functions

func extractTraceIDFromContext(ctx context.Context) string {
	// This would be implemented with OpenTelemetry integration
	return ""
}

func extractSpanIDFromContext(ctx context.Context) string {
	// This would be implemented with OpenTelemetry integration
	return ""
}

// StdoutSink for compatibility
type StdoutSink struct {
	formatter outbound.Formatter
}

// NewStdoutSink creates stdout sink
func NewStdoutSink() outbound.Sink {
	return &StdoutSink{
		formatter: NewJSONFormatter(),
	}
}

func (s *StdoutSink) Write(entry *domain.LogEntry) error {
	fmt.Println(s.formatter.Format(entry))
	return nil
}

func (s *StdoutSink) Close() error {
	return nil
}

func (s *StdoutSink) Name() string {
	return "stdout"
}

func (s *StdoutSink) Health() error {
	return nil
}

// JSONFormatter for compatibility
type JSONFormatter struct{}

func NewJSONFormatter() outbound.Formatter {
	return &JSONFormatter{}
}

func (f *JSONFormatter) Format(entry *domain.LogEntry) string {
	// Simple JSON formatting
	return fmt.Sprintf(`{"level":"%s","message":"%s","timestamp":"%s"}`,
		entry.Level.String(), entry.Message, entry.Timestamp.Format(time.RFC3339))
}

func (f *JSONFormatter) Name() string {
	return "json"
}

func (f *JSONFormatter) Configure(config map[string]interface{}) error {
	return nil
}

// Package core provides the unified best-performance logBrt implementation
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"obs-brutal/internal/core/domain"

	"gopkg.in/natefinch/lumberjack.v2"
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

// NewSmartLogBrtWithOptions returns a logger based on options (compat for facade)
func NewSmartLogBrtWithOptions(opts ...ConfigOption) LogBrt {
	config := NewSmartConfig(opts...)
	if config.HasOTEL() && config.HasSecurity() {
		if enterprise, err := NewEnterpriseLogBrt(
			config.GetServiceName(),
			config.GetVersion(),
			config.GetEnvironment(),
			config.GetOTELEndpoint(),
			config.options.LogLevel,
		); err == nil {
			return enterprise
		}
	} else if config.HasOTEL() {
		if otel, err := NewOTelLogBrt(
			config.GetServiceName(),
			config.GetVersion(),
			config.GetEnvironment(),
			config.GetOTELEndpoint(),
			config.options.LogLevel,
		); err == nil {
			return otel
		}
	} else if config.HasAsync() {
		return NewAsyncLogBrt(config.options.LogLevel)
	}
	return NewUnifiedLogBrt(config.options.LogLevel)
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
	// rotation options
	rotateBySizeBytes int64
	maxBackups        int
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

	// rotate if needed
	if s.rotateBySizeBytes > 0 {
		if fi, err := s.file.Stat(); err == nil {
			if fi.Size() >= s.rotateBySizeBytes {
				_ = s.file.Close()
				s.rotateFiles()
				f, err := os.OpenFile(s.filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
				if err != nil {
					return err
				}
				s.file = f
			}
		}
	}

	return writeJSONToWriter(s.file, entry)
}

func (s *OptimalFileSink) rotateFiles() {
	// simple backups: file -> file.1 -> file.2 ...
	for i := s.maxBackups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", s.filename, i)
		newp := fmt.Sprintf("%s.%d", s.filename, i+1)
		_ = os.Rename(old, newp)
	}
	_ = os.Rename(s.filename, fmt.Sprintf("%s.%d", s.filename, 1))
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

func (s *OptimalFileSink) Configure(config map[string]interface{}) error {
	if v, ok := config["filename"].(string); ok && v != "" {
		s.filename = v
	}
	if v, ok := config["rotate_size_bytes"].(int64); ok && v > 0 {
		s.rotateBySizeBytes = v
	}
	if v, ok := config["max_backups"].(int); ok && v > 0 {
		s.maxBackups = v
	}
	return nil
}

// JSONSink provides pure JSON output
type JSONSink struct{ SinkBase }

func (s *JSONSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	return writeJSONToWriter(os.Stdout, entry)
}

func (s *JSONSink) Name() string { return "json" }

// ===== TOGGLE SINK (enable/disable wrapper) =====

type ToggleSink struct {
	SinkBase
	inner   Sink
	enabled atomic.Bool
}

func NewToggleSink(inner Sink, enabled bool) Sink {
	t := &ToggleSink{inner: inner}
	t.enabled.Store(enabled)
	return t
}

func (t *ToggleSink) Write(entry *domain.LogEntry) error {
	if entry == nil || !t.enabled.Load() || t.inner == nil {
		return nil
	}
	return t.inner.Write(entry)
}

func (t *ToggleSink) Name() string {
	if t.inner == nil {
		return "toggle(nil)"
	}
	return "toggle(" + t.inner.Name() + ")"
}

func (t *ToggleSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["enabled"].(bool); ok {
		t.enabled.Store(v)
	}
	// pass-through configuration to inner if provided
	if t.inner != nil {
		_ = t.inner.Configure(cfg)
	}
	return nil
}

// NewConsoleSink wraps stdout sink with toggle
func NewConsoleSink(enabled bool) Sink { return NewToggleSink(NewFastStdoutSink(), enabled) }

// ===== LUMBERJACK FILE SINK =====

type LumberjackSink struct {
	SinkBase
	lj *lumberjack.Logger
	mu sync.Mutex
	// config cache
	filename   string
	maxSizeMB  int
	maxBackups int
	maxAgeDays int
	compress   bool
}

func NewLumberjackSink() Sink {
	return &LumberjackSink{
		filename:   "logs/app.log",
		maxSizeMB:  50,
		maxBackups: 7,
		maxAgeDays: 7,
		compress:   true,
	}
}

func (s *LumberjackSink) ensure() {
	if s.lj != nil {
		return
	}
	s.lj = &lumberjack.Logger{
		Filename:   s.filename,
		MaxSize:    s.maxSizeMB,
		MaxBackups: s.maxBackups,
		MaxAge:     s.maxAgeDays,
		Compress:   s.compress,
	}
}

func (s *LumberjackSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.mu.Lock()
	s.ensure()
	s.mu.Unlock()
	b, err := encodeEntryToJSON(entry)
	if err != nil {
		return err
	}
	_, err = s.lj.Write(b)
	return err
}

func (s *LumberjackSink) Name() string { return "lumberjack" }

func (s *LumberjackSink) Configure(cfg map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := cfg["filename"].(string); ok && v != "" {
		s.filename = v
	}
	if v, ok := cfg["max_size_mb"].(int); ok && v > 0 {
		s.maxSizeMB = v
	}
	if v, ok := cfg["max_backups"].(int); ok && v >= 0 {
		s.maxBackups = v
	}
	if v, ok := cfg["max_age_days"].(int); ok && v >= 0 {
		s.maxAgeDays = v
	}
	if v, ok := cfg["compress"].(bool); ok {
		s.compress = v
	}
	s.lj = nil // recreate on next write
	return nil
}

// ===== CLICKHOUSE SINK (HTTP JSONEachRow) =====

type ClickHouseSink struct {
	SinkBase
	endpoint   string
	database   string
	table      string
	username   string
	password   string
	autoCreate bool
	client     *http.Client
	initOnce   sync.Once
}

func NewClickHouseSink() Sink {
	return &ClickHouseSink{
		endpoint:   "http://localhost:8123",
		database:   "default",
		table:      "logs",
		autoCreate: true,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *ClickHouseSink) Name() string { return "clickhouse" }

func (s *ClickHouseSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := cfg["database"].(string); ok && v != "" {
		s.database = v
	}
	if v, ok := cfg["table"].(string); ok && v != "" {
		s.table = v
	}
	if v, ok := cfg["username"].(string); ok {
		s.username = v
	}
	if v, ok := cfg["password"].(string); ok {
		s.password = v
	}
	if v, ok := cfg["auto_create"].(bool); ok {
		s.autoCreate = v
	}
	return nil
}

func (s *ClickHouseSink) ensureTable() {
	if !s.autoCreate {
		return
	}
	create := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
        datetime DateTime,
        level LowCardinality(String),
        msg String,
        trace_id String, span_id String, request_id String, user_id String,
        module String, tenant_id String, error String,
        fields String
    ) ENGINE = MergeTree ORDER BY (datetime, level)`, s.database, s.table)
	_ = s.execQuery(create)
}

func (s *ClickHouseSink) execQuery(query string) error {
	u, _ := url.Parse(s.endpoint)
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest("POST", u.String(), nil)
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse status: %s", resp.Status)
	}
	return nil
}

func (s *ClickHouseSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.initOnce.Do(s.ensureTable)
	// build row JSON matching table schema
	dt := entry.Timestamp.Format("2006-01-02 15:04:05")
	traceID := entry.TraceID
	if traceID == "" {
		if v, ok := entry.Fields["trace_id"].(string); ok {
			traceID = v
		}
	}
	spanID := entry.SpanID
	if spanID == "" {
		if v, ok := entry.Fields["span_id"].(string); ok {
			spanID = v
		}
	}
	requestID := entry.RequestID
	if requestID == "" {
		if v, ok := entry.Fields["request_id"].(string); ok {
			requestID = v
		}
	}
	userID := entry.UserID
	if userID == "" {
		if v, ok := entry.Fields["user_id"].(string); ok {
			userID = v
		}
	}
	module := entry.Module
	if module == "" {
		if v, ok := entry.Fields["module"].(string); ok {
			module = v
		}
	}
	tenant := entry.TenantID
	if tenant == "" {
		if v, ok := entry.Fields["tenant_id"].(string); ok {
			tenant = v
		}
	}
	errStr := ""
	if entry.Error != nil {
		errStr = entry.Error.Error()
	} else if v, ok := entry.Fields["error"].(string); ok {
		errStr = v
	}
	fieldsJSON, _ := json.Marshal(entry.Fields)
	rowMap := map[string]interface{}{
		"datetime":   dt,
		"level":      entry.Level.String(),
		"msg":        entry.Message,
		"trace_id":   traceID,
		"span_id":    spanID,
		"request_id": requestID,
		"user_id":    userID,
		"module":     module,
		"tenant_id":  tenant,
		"error":      errStr,
		"fields":     string(fieldsJSON),
	}
	row, err := json.Marshal(rowMap)
	if err != nil {
		return err
	}
	u, _ := url.Parse(s.endpoint)
	q := u.Query()
	q.Set("query", fmt.Sprintf("INSERT INTO %s.%s FORMAT JSONEachRow", s.database, s.table))
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest("POST", u.String(), bytes.NewReader(row))
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse insert: %s", resp.Status)
	}
	return nil
}

// ===== ALERT SINKS WITH SIMPLE RETRY/BACKOFF =====

// retry with exponential backoff (max 5 attempts, starting 200ms)
func retryBackoff(op func() error) error {
	delay := 200 * time.Millisecond
	for attempt := 0; attempt < 5; attempt++ {
		if err := op(); err != nil {
			if attempt == 4 {
				return err
			}
			time.Sleep(delay)
			if delay < 2*time.Second {
				delay *= 2
			}
			continue
		}
		return nil
	}
	return nil
}

// SlackSink posts to Slack Incoming Webhook
type SlackSink struct {
	SinkBase
	webhookURL string
	client     *http.Client
}

func NewSlackSink(webhookURL string) Sink {
	return &SlackSink{webhookURL: webhookURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (s *SlackSink) Name() string { return "slack" }

func (s *SlackSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["webhook_url"].(string); ok && v != "" {
		s.webhookURL = v
	}
	return nil
}

func (s *SlackSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.webhookURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"text": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Message),
	}
	body, _ := json.Marshal(payload)
	return retryBackoff(func() error {
		req, _ := http.NewRequest("POST", s.webhookURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("slack status: %s", resp.Status)
		}
		return nil
	})
}

// TelegramSink posts to Telegram bot API sendMessage
type TelegramSink struct {
	SinkBase
	botToken string
	chatID   string
	client   *http.Client
}

func NewTelegramSink(botToken, chatID string) Sink {
	return &TelegramSink{botToken: botToken, chatID: chatID, client: &http.Client{Timeout: 5 * time.Second}}
}

func (s *TelegramSink) Name() string { return "telegram" }

func (s *TelegramSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["bot_token"].(string); ok {
		s.botToken = v
	}
	if v, ok := cfg["chat_id"].(string); ok {
		s.chatID = v
	}
	return nil
}

func (s *TelegramSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.botToken == "" || s.chatID == "" {
		return nil
	}
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	data := map[string]string{"chat_id": s.chatID, "text": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Message)}
	body, _ := json.Marshal(data)
	return retryBackoff(func() error {
		req, _ := http.NewRequest("POST", api, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("telegram status: %s", resp.Status)
		}
		return nil
	})
}

// OpsgenieSink creates alert via Opsgenie API
type OpsgenieSink struct {
	SinkBase
	apiKey   string
	endpoint string
	client   *http.Client
	priority string
}

func NewOpsgenieSink(apiKey string) Sink {
	return &OpsgenieSink{apiKey: apiKey, endpoint: "https://api.opsgenie.com/v2/alerts", client: &http.Client{Timeout: 5 * time.Second}, priority: "P3"}
}

func (s *OpsgenieSink) Name() string { return "opsgenie" }

func (s *OpsgenieSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["api_key"].(string); ok {
		s.apiKey = v
	}
	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := cfg["priority"].(string); ok && v != "" {
		s.priority = v
	}
	return nil
}

func (s *OpsgenieSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.apiKey == "" {
		return nil
	}
	payload := map[string]interface{}{
		"message":  fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Message),
		"priority": s.priority,
	}
	body, _ := json.Marshal(payload)
	return retryBackoff(func() error {
		req, _ := http.NewRequest("POST", s.endpoint, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "GenieKey "+s.apiKey)
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("opsgenie status: %s", resp.Status)
		}
		return nil
	})
}

// BufferedSink provides high-throughput logging
type BufferedSink struct {
	SinkBase
	buffer    []*domain.LogEntry
	maxSize   int
	timeout   time.Duration
	lastFlush time.Time
	mu        sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
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
	// signal stop and wait for flusher to exit
	s.mu.Lock()
	if s.stopCh != nil {
		close(s.stopCh)
	}
	s.mu.Unlock()

	s.wg.Wait()

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

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
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

// encodeEntryToJSON returns JSON bytes for a log entry (without writing)
func encodeEntryToJSON(entry *domain.LogEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeJSONToWriter(&buf, entry); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ===== LOKI PUSH SINK =====

// LokiPushSink sends logs to Loki HTTP push API with labels.
// Minimal payload for compatibility with Promtail/Agent.
type LokiPushSink struct {
	SinkBase
	endpoint string
	labels   map[string]string
	client   *http.Client
}

func NewLokiPushSink(endpoint string, labels map[string]string) *LokiPushSink {
	if labels == nil {
		labels = map[string]string{"app": "obs-brutal"}
	}
	return &LokiPushSink{
		endpoint: endpoint,
		labels:   labels,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *LokiPushSink) Name() string { return "loki" }

func (s *LokiPushSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := cfg["labels"].(map[string]string); ok {
		s.labels = v
	}
	return nil
}

func (s *LokiPushSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.endpoint == "" {
		return nil
	}

	// Build streams payload
	// { "streams": [ { "stream": {label...}, "values": [[ts, line]] } ] }
	lineBytes, err := encodeEntryToJSON(entry)
	if err != nil {
		return err
	}
	ts := entry.Timestamp.UnixNano()
	payload := map[string]interface{}{
		"streams": []map[string]interface{}{
			{
				"stream": s.labels,
				"values": [][]string{{fmt.Sprintf("%d", ts), string(lineBytes)}},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("loki push failed: %s", resp.Status)
	}
	return nil
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

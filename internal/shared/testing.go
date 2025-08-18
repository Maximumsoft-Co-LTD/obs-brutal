package shared

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	pin "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
)

// MockLogger provides a mock logger for testing
type MockLogger struct {
	mu       sync.RWMutex
	logs     []MockLogEntry
	fields   map[string]interface{}
	level    domain.Level
	filtered int64
}

// MockLogEntry represents a logged entry
type MockLogEntry struct {
	Level     domain.Level
	Message   string
	Fields    map[string]interface{}
	Timestamp time.Time
	Error     error
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *MockLogger {
	return &MockLogger{
		logs:   make([]MockLogEntry, 0),
		fields: make(map[string]interface{}),
		level:  domain.DebugLevel,
	}
}

// Ctx implements Logger interface
func (m *MockLogger) Ctx(ctx context.Context) pin.Logger {
	// Extract context values if needed
	return m
}

// F implements Logger interface
func (m *MockLogger) F(key string, value interface{}) pin.Logger {
	m.mu.Lock()
	defer m.mu.Unlock()

	newMock := &MockLogger{
		logs:   m.logs,
		fields: make(map[string]interface{}),
		level:  m.level,
	}

	// Copy existing fields
	for k, v := range m.fields {
		newMock.fields[k] = v
	}
	newMock.fields[key] = value

	return newMock
}

// Fs implements Logger interface
func (m *MockLogger) Fs(fields map[string]interface{}) pin.Logger {
	newMock := m
	for k, v := range fields {
		newMock = newMock.F(k, v).(*MockLogger)
	}
	return newMock
}

// Err implements Logger interface
func (m *MockLogger) Err(err error) pin.Logger {
	return m.F("error", err.Error())
}

// TID implements Logger interface
func (m *MockLogger) TID(traceID string) pin.Logger {
	return m.F("trace_id", traceID)
}

// SID implements Logger interface
func (m *MockLogger) SID(spanID string) pin.Logger {
	return m.F("span_id", spanID)
}

// UID implements Logger interface
func (m *MockLogger) UID(userID string) pin.Logger {
	return m.F("user_id", userID)
}

// RID implements Logger interface
func (m *MockLogger) RID(requestID string) pin.Logger {
	return m.F("request_id", requestID)
}

// IP implements Logger interface
func (m *MockLogger) IP(ip string) pin.Logger {
	return m.F("client_ip", ip)
}

// Sess implements Logger interface
func (m *MockLogger) Sess(sessionID string) pin.Logger {
	return m.F("session_id", sessionID)
}

// Tenant implements Logger interface
func (m *MockLogger) Tenant(tenant string) pin.Logger {
	return m.F("tenant_id", tenant)
}

// Mod implements Logger interface
func (m *MockLogger) Mod(module string) pin.Logger {
	return m.F("module", module)
}

// log helper
func (m *MockLogger) log(level domain.Level, msg string, fields ...domain.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if level < m.level {
		m.filtered++
		return
	}

	entry := MockLogEntry{
		Level:     level,
		Message:   msg,
		Fields:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Copy fields
	for k, v := range m.fields {
		entry.Fields[k] = v
	}

	// Add extra fields
	for _, f := range fields {
		entry.Fields[f.Key] = f.Value
	}

	// Extract error if present
	if err, ok := entry.Fields["error"]; ok {
		if errStr, ok := err.(string); ok {
			entry.Error = fmt.Errorf("%s", errStr)
		}
	}

	m.logs = append(m.logs, entry)
}

// Debug implements Logger interface
func (m *MockLogger) Debug(msg string, fields ...domain.Field) {
	m.log(domain.DebugLevel, msg, fields...)
}

// Info implements Logger interface
func (m *MockLogger) Info(msg string, fields ...domain.Field) {
	m.log(domain.InfoLevel, msg, fields...)
}

// Warn implements Logger interface
func (m *MockLogger) Warn(msg string, fields ...domain.Field) {
	m.log(domain.WarnLevel, msg, fields...)
}

// Error implements Logger interface
func (m *MockLogger) Error(msg string, fields ...domain.Field) {
	m.log(domain.ErrorLevel, msg, fields...)
}

// Fatal implements Logger interface
func (m *MockLogger) Fatal(msg string, fields ...domain.Field) {
	m.log(domain.FatalLevel, msg, fields...)
}

// Level implements Logger interface
func (m *MockLogger) Level(level domain.Level) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.level = level
}

// GetLevel implements Logger interface
func (m *MockLogger) GetLevel() domain.Level {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.level
}

// Logged implements Logger interface
func (m *MockLogger) Logged() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.logs))
}

// Filtered implements Logger interface
func (m *MockLogger) Filtered() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.filtered
}

// Testing assertions

// GetLogs returns all logged entries
func (m *MockLogger) GetLogs() []MockLogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logs := make([]MockLogEntry, len(m.logs))
	copy(logs, m.logs)
	return logs
}

// AssertLogged asserts that a log entry exists
func (m *MockLogger) AssertLogged(t *testing.T, level domain.Level, message string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, log := range m.logs {
		if log.Level == level && strings.Contains(log.Message, message) {
			return
		}
	}

	t.Errorf("Expected log not found. Level: %v, Message containing: %s", level, message)
	t.Logf("Actual logs:")
	for _, log := range m.logs {
		t.Logf("  [%v] %s", log.Level, log.Message)
	}
}

// AssertLoggedF asserts that a log entry exists with a specific field
func (m *MockLogger) AssertLoggedF(t *testing.T, level domain.Level, key string, value interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, log := range m.logs {
		if log.Level == level {
			if val, ok := log.Fields[key]; ok && val == value {
				return
			}
		}
	}

	t.Errorf("Expected log with field not found. Level: %v, Field: %s=%v", level, key, value)
}

// AssertNotLogged asserts that a log entry does not exist
func (m *MockLogger) AssertNotLogged(t *testing.T, level domain.Level, message string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, log := range m.logs {
		if log.Level == level && strings.Contains(log.Message, message) {
			t.Errorf("Unexpected log found. Level: %v, Message: %s", level, log.Message)
			return
		}
	}
}

// AssertLogCount asserts the number of logs at a specific level
func (m *MockLogger) AssertLogCount(t *testing.T, level domain.Level, expectedCount int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, log := range m.logs {
		if log.Level == level {
			count++
		}
	}

	if count != expectedCount {
		t.Errorf("Expected %d logs at level %v, but got %d", expectedCount, level, count)
	}
}

// Clear clears all logged entries
func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = make([]MockLogEntry, 0)
}

// LastLog returns the last logged entry
func (m *MockLogger) LastLog() *MockLogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.logs) == 0 {
		return nil
	}

	entry := m.logs[len(m.logs)-1]
	return &entry
}

// TestLoggerFactory creates loggers for testing
type TestLoggerFactory struct {
	loggers map[string]*MockLogger
	mu      sync.RWMutex
}

// NewTestLoggerFactory creates a new test logger factory
func NewTestLoggerFactory() *TestLoggerFactory {
	return &TestLoggerFactory{
		loggers: make(map[string]*MockLogger),
	}
}

// GetLogger gets or creates a logger for a module
func (f *TestLoggerFactory) GetLogger(module string) *MockLogger {
	f.mu.Lock()
	defer f.mu.Unlock()

	if logger, ok := f.loggers[module]; ok {
		return logger
	}

	logger := NewMockLogger()
	f.loggers[module] = logger
	return logger
}

// GetAllLogs returns all logs from all modules
func (f *TestLoggerFactory) GetAllLogs() map[string][]MockLogEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	allLogs := make(map[string][]MockLogEntry)
	for module, logger := range f.loggers {
		allLogs[module] = logger.GetLogs()
	}
	return allLogs
}

// LogCapture captures logs during test execution
type LogCapture struct {
	original pin.Logger
	mock     *MockLogger
}

// CaptureLogger captures logs from a logger
func CaptureLogger(logger pin.Logger) *LogCapture {
	mock := NewMockLogger()
	return &LogCapture{
		original: logger,
		mock:     mock,
	}
}

// Logger returns the mock logger to use
func (c *LogCapture) Logger() pin.Logger {
	return c.mock
}

// AssertLogged asserts logs were captured
func (c *LogCapture) AssertLogged(t *testing.T, level domain.Level, message string) {
	c.mock.AssertLogged(t, level, message)
}

// GetLogs returns captured logs
func (c *LogCapture) GetLogs() []MockLogEntry {
	return c.mock.GetLogs()
}

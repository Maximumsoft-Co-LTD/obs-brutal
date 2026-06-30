package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"
)

// Level represents log severity level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns string representation of level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevelString converts string to Level (case-insensitive)
func ParseLevelString(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "fatal":
		return FatalLevel
	default:
		return InfoLevel
	}
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// LogEntry represents a single log entry
type LogEntry struct {
	Level     Level
	Msg       string
	F         map[string]interface{}
	Timestamp time.Time
	Mod       string
	TenantID  string
	UserID    string
	TraceID   string
	SpanID    string
	RequestID string
	Err       error
}

// StructuredError represents a structured error
type StructuredError struct {
	Code     string
	Message  string
	Category string
	Details  map[string]interface{}
	Cause    error
}

// Error implements error interface
func (e *StructuredError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// GetCode returns error code
func (e *StructuredError) GetCode() string { return e.Code }

// GetCategory returns error category
func (e *StructuredError) GetCategory() string { return e.Category }

// GetMessage returns error message
func (e *StructuredError) GetMessage() string { return e.Message }

// GetDetails returns error details
func (e *StructuredError) GetDetails() map[string]interface{} { return e.Details }

// GetCause returns error cause
func (e *StructuredError) GetCause() error { return e.Cause }

// FilterRule represents a filter rule
type FilterRule struct {
	ID       string
	Type     string
	Pattern  string
	Module   string
	TenantID string
	Level    Level
	Enabled  bool
	Config   map[string]interface{}
}

// ===== ENHANCED UTILITY FUNCTIONS =====

var fallbackRandSeed sync.Once

// GenerateID creates efficient IDs using Go 1.25 optimizations
func GenerateID(prefix string) string {
	// Use timestamp + random for uniqueness and performance
	timestamp := time.Now().UnixNano()

	// Generate 4 random bytes
	randomBytes := make([]byte, 4)
	randomHex := ""
	if _, err := rand.Read(randomBytes); err == nil {
		randomHex = hex.EncodeToString(randomBytes)
	} else {
		fallbackRandSeed.Do(func() {
			mathrand.Seed(time.Now().UnixNano())
		})
		randomHex = fmt.Sprintf("%08x", mathrand.Int63()&0xffffffff)
	}

	return fmt.Sprintf("%s_%d_%s", prefix, timestamp&0xFFFFFFFF, randomHex)
}

// NewLogEntry creates optimized log entry with Go 1.25 features
func NewLogEntry() *LogEntry {
	return &LogEntry{
		F:         make(map[string]interface{}, 8), // Pre-allocate
		Timestamp: time.Now(),
	}
}

// Clone creates a deep copy of log entry
func (le *LogEntry) Clone() *LogEntry {
	clone := &LogEntry{
		Level:     le.Level,
		Msg:       le.Msg,
		Timestamp: le.Timestamp,
		Mod:       le.Mod,
		TenantID:  le.TenantID,
		UserID:    le.UserID,
		TraceID:   le.TraceID,
		SpanID:    le.SpanID,
		RequestID: le.RequestID,
		Err:       le.Err,
		F:         make(map[string]interface{}, len(le.F)),
	}

	// Copy fields efficiently
	for k, v := range le.F {
		clone.F[k] = v
	}

	return clone
}

// Reset resets log entry for reuse (Go 1.25 optimized)
func (le *LogEntry) Reset() {
	le.Level = InfoLevel
	le.Msg = ""
	le.Timestamp = time.Time{}
	le.Mod = ""
	le.TenantID = ""
	le.UserID = ""
	le.TraceID = ""
	le.SpanID = ""
	le.RequestID = ""
	le.Err = nil

	// Go 1.25: Use clear() for efficient map reset
	clear(le.F)
}

// AddField adds a field to the log entry
func (le *LogEntry) AddField(key string, value interface{}) {
	if le.F == nil {
		le.F = make(map[string]interface{}, 8)
	}
	le.F[key] = value
}

// AddFields adds multiple fields efficiently
func (le *LogEntry) AddFields(fields map[string]interface{}) {
	if le.F == nil {
		le.F = make(map[string]interface{}, len(fields))
	}

	for k, v := range fields {
		le.F[k] = v
	}
}

// HasField checks if field exists
func (le *LogEntry) HasField(key string) bool {
	_, exists := le.F[key]
	return exists
}

// GetField gets field value with type assertion
func (le *LogEntry) GetField(key string) (interface{}, bool) {
	value, exists := le.F[key]
	return value, exists
}

// GetStringField gets string field value
func (le *LogEntry) GetStringField(key string) (string, bool) {
	if value, exists := le.F[key]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// ===== ENHANCED ERROR TYPES =====

// NewHTTPError creates HTTP error for span recording
func NewHTTPError(statusCode int, message string) error {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// HTTPError represents HTTP-related errors
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// ===== PERFORMANCE UTILITIES =====

// LogEntryPool provides object pooling for log entries
type LogEntryPool struct {
	pool chan *LogEntry
	size int
}

// NewLogEntryPool creates optimized log entry pool
func NewLogEntryPool(size int) *LogEntryPool {
	pool := &LogEntryPool{
		pool: make(chan *LogEntry, size),
		size: size,
	}

	// Pre-fill pool
	for i := 0; i < size; i++ {
		pool.pool <- NewLogEntry()
	}

	return pool
}

// Get retrieves log entry from pool
func (p *LogEntryPool) Get() *LogEntry {
	select {
	case entry := <-p.pool:
		return entry
	default:
		// Pool empty, create new
		return NewLogEntry()
	}
}

// Put returns log entry to pool
func (p *LogEntryPool) Put(entry *LogEntry) {
	if entry == nil {
		return
	}

	// Reset for reuse
	entry.Reset()

	select {
	case p.pool <- entry:
		// Successfully returned to pool
	default:
		// Pool full, let GC handle it
	}
}

// ===== GLOBAL POOLS (Go 1.25 optimized) =====

var (
	// Global log entry pool for maximum performance
	GlobalLogEntryPool = NewLogEntryPool(1000)

	// Global field map pool
	FieldMapPool = make(chan map[string]interface{}, 100)
)

// GetFieldMap gets field map from pool
func GetFieldMap() map[string]interface{} {
	select {
	case fields := <-FieldMapPool:
		return fields
	default:
		return make(map[string]interface{}, 8)
	}
}

// PutFieldMap returns field map to pool
func PutFieldMap(fields map[string]interface{}) {
	if fields == nil {
		return
	}

	// Go 1.25: Use clear() for efficient reset
	clear(fields)

	select {
	case FieldMapPool <- fields:
		// Successfully returned to pool
	default:
		// Pool full, let GC handle it
	}
}

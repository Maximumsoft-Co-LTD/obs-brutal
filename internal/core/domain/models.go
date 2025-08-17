package domain

import (
	"strings"
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
	Message   string
	Fields    map[string]interface{}
	Timestamp time.Time
	Module    string
	TenantID  string
	UserID    string
	TraceID   string
	SpanID    string
	RequestID string
	Error     error
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

// ResponseOption configures response builder
type ResponseOption func(*ResponseConfig)

// ResponseConfig holds response configuration
type ResponseConfig struct {
	Message string
	Fields  map[string]interface{}
}

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

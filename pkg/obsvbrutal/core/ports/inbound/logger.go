package inbound

import (
	"context"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Logger is the main inbound port for logging operations
// This is what the application uses to log
type Logger interface {
	// Context methods
	Ctx(ctx context.Context) Logger
	F(key string, value interface{}) Logger
	Fs(fields map[string]interface{}) Logger
	Err(err error) Logger

	// Correlation IDs
	TID(traceID string) Logger
	SID(spanID string) Logger
	UID(userID string) Logger
	RID(requestID string) Logger
	IP(ip string) Logger
	Sess(sessionID string) Logger
	Tenant(tenant string) Logger
	Mod(module string) Logger

	// Logging methods
	Debug(msg string, fields ...domain.Field)
	Info(msg string, fields ...domain.Field)
	Warn(msg string, fields ...domain.Field)
	Error(msg string, fields ...domain.Field)
	Fatal(msg string, fields ...domain.Field)

	// Configuration
	Level(level domain.Level)
	GetLevel() domain.Level

	// Metrics
	Logged() int64
	Filtered() int64
}

// Simple provides a fluent interface for easier logging
type Simple interface {
	// Field management
	F(key string, value interface{}) Simple
	Fs(fields map[string]interface{}) Simple

	// Logging methods
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Prt(format string, args ...interface{})

	// Error handling
	Err(err error) Simple
	EC(err error, category string) Simple

	// Development
	Dev() Simple
	Bench(name string) func()
	Lat(operation string, start time.Time)

	// Structured errors
	StructErr(err domain.StructuredError) Simple

	// Response building
	R(status int, opts ...domain.ResponseOption) Response

	// Tracing
	Parent(name string) Trace
	Close()
}

// Trace represents a parent trace/span
type Trace interface {
	// Child traces
	Parent(name string) Trace

	// Attributes
	Add(attrs ...attribute.KeyValue)
	Str(key, value string) attribute.KeyValue
	Bool(key string, value bool) attribute.KeyValue
	Num(key string, value float64) attribute.KeyValue
	Body(key string, value interface{}) attribute.KeyValue
	Detail(msg string) attribute.KeyValue
	Msg(msg string) attribute.KeyValue
	Code(code int) attribute.KeyValue

	// Error handling
	Err(err error) error
	Errf(format string, args ...interface{}) error

	// Lifecycle
	End()
}

// Response for building HTTP responses with logging
type Response interface {
	AddField(key string, value interface{}) Response
	Msg(msg string) Response
	Err(err error) error
	Errf(format string, args ...interface{}) error
	Send(status int, opts ...domain.ResponseOption) error
}

// Service is the main use case interface
type Service interface {
	// Log with context and features
	Log(ctx context.Context, level domain.Level, msg string, fields map[string]interface{}) error

	// Log error with category
	LogErr(ctx context.Context, err error, category string, details map[string]interface{}) error

	// Log structured error
	LogStruct(ctx context.Context, err domain.StructuredError) error

	// Create logger with configuration
	New(config Config) (Logger, error)

	// Create simplified logger
	NewSimple(ctx context.Context, operationName string) (Simple, error)
}

// Configuration Management Ports

// Features manages custom features
type Features interface {
	Register(name string, feature Feature) error
	Get(name string) (Feature, error)
	List() []string
	Apply(logger Logger, features []string) Logger
}

// ErrCategories manages error categories
type ErrCategories interface {
	Register(category string, handler ErrHandler) error
	Get(category string) (ErrHandler, error)
	List() []string
	Handle(logger Logger, err error, category string, details map[string]interface{})
}

// Feature represents a custom logging feature
type Feature interface {
	Name() string
	Apply(logger Logger) Logger
	Configure(config map[string]interface{}) error
}

// ErrHandler handles specific error categories
type ErrHandler interface {
	Category() string
	Handle(logger Logger, err error, details map[string]interface{})
	ShouldAlert() bool
	Severity() domain.Level
}

// Configuration Types

// Config for logger configuration
type Config struct {
	Level   domain.Level
	Svc     string   // Service name
	Env     string   // Environment
	Sinks   []string // Names of sinks to use
	Filters []string // Names of filters to use
	Sampler string   // Name of sampler to use
	Format  string   // Name of formatter to use
	BufSize int
	Async   bool
	Feat    []string               // Features
	Cfg     map[string]interface{} // Custom config
}

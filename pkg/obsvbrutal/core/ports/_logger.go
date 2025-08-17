package ports

import (
	"context"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Logger is the main logging port
type Logger interface {
	// Context methods
	Ctx(ctx context.Context) Logger
	F(key string, value interface{}) Logger
	FBatch(fields map[string]interface{}) Logger
	Err(err error) Logger

	// Correlation IDs
	TraceID(traceID string) Logger
	SpanID(spanID string) Logger
	UserID(userID string) Logger
	RequestID(requestID string) Logger
	ClientIP(ip string) Logger
	SessionID(sessionID string) Logger
	Tenant(tenant string) Logger
	Module(module string) Logger

	// Logging methods
	Debug(msg string, fields ...domain.Field) error
	Info(msg string, fields ...domain.Field) error
	Warn(msg string, fields ...domain.Field) error
	Error(err error, fields ...domain.Field) error
	Fatal(msg string, fields ...domain.Field) error
	Errf(format string, args ...interface{}) error
	ErrCat(err error, category string) error
	Prt(format string, args ...interface{}) error
	Prtf(format string, args ...interface{}) error
	Benchmark(name string) func()
	RecordLatency(operation string, start time.Time)

	// Configuration
	Filter(filter FilterStrategy)
	Sampler(sampler SamplerStrategy)
	Formatter(formatter FormatterStrategy)
	Level(level domain.Level)
	GetLevel() domain.Level

	// Metrics
	LoggedCount() int64
	FilteredCount() int64
}

// ParentTracer represents a parent trace/span
type ParentTracer interface {
	// Child traces
	Parent(name string) ParentTracer

	// Attributes
	Prt(attrs ...attribute.KeyValue)
	Wpt()
	Str(key, value string) attribute.KeyValue
	Bool(key string, value bool) attribute.KeyValue
	Num(key string, value float64) attribute.KeyValue
	Body(key string, value interface{}) attribute.KeyValue
	Err(err error) attribute.KeyValue
	Detail(msg string) attribute.KeyValue
	Msg(msg string) attribute.KeyValue
	Code(code int) attribute.KeyValue

	// Error handling
	Errf(format string, args ...interface{}) error

	// Lifecycle
	End()
}

// StructuredErrorInterface for structured errors
type StructuredErrorInterface interface {
	error
	GetCode() string
	GetCategory() string
	GetMessage() string
	GetDetails() map[string]interface{}
	GetCause() error
}

// Strategy Ports

// FilterStrategy defines log filtering
type FilterStrategy interface {
	Filter(entry *domain.LogEntry) *domain.LogEntry
	Name() string
}

// SamplerStrategy defines log sampling
type SamplerStrategy interface {
	Sample(entry *domain.LogEntry) bool
	Name() string
}

// FormatterStrategy defines log formatting
type FormatterStrategy interface {
	Format(entry *domain.LogEntry) string
	Name() string
}

// Sink defines log output
type Sink interface {
	Write(entry *domain.LogEntry) error
	Close() error
	Name() string
}

// ConfigProvider provides configuration
type ConfigProvider interface {
	GetLogLevel(module, tenant string) domain.Level
	GetFilterRules() []domain.FilterRule
	GetSamplingRate(module string) float32
	Subscribe(callback func()) error
	Close() error
}

// Registry Ports

// FeatureRegistry manages features
type FeatureRegistry interface {
	RegisterFeature(name string, feature Feature)
	GetFeature(name string) (Feature, bool)
	ListFeatures() []string
}

// Feature represents a logging feature
type Feature interface {
	Name() string
	Apply(logger Logger) Logger
	Configure(config map[string]interface{}) error
}

// ErrorCategoryRegistry manages error categories
type ErrorCategoryRegistry interface {
	RegisterCategory(category string, handler ErrorCategoryHandler)
	GetHandler(category string) (ErrorCategoryHandler, bool)
	ListCategories() []string
}

// ErrorCategoryHandler handles error categories
type ErrorCategoryHandler interface {
	Category() string
	Handle(logger Logger, err error, details map[string]interface{})
	ShouldAlert() bool
	GetSeverity() domain.Level
}

// Factory Ports

// LoggerFactory creates loggers
type LoggerFactory interface {
	CreateLogger(config LoggerConfig) (Logger, error)
}

// SinkFactory creates sinks
type SinkFactory interface {
	CreateSink(config SinkConfig) (Sink, error)
	RegisterSinkType(name string, creator SinkCreator)
}

// StrategyFactory creates strategies
type StrategyFactory interface {
	CreateFilter(name string, config map[string]interface{}) (FilterStrategy, error)
	CreateSampler(name string, config map[string]interface{}) (SamplerStrategy, error)
	CreateFormatter(name string, config map[string]interface{}) (FormatterStrategy, error)

	RegisterFilter(name string, creator FilterCreator)
	RegisterSampler(name string, creator SamplerCreator)
	RegisterFormatter(name string, creator FormatterCreator)
}

// Creator Types
type (
	SinkCreator      func(config map[string]interface{}) (Sink, error)
	FilterCreator    func(config map[string]interface{}) (FilterStrategy, error)
	SamplerCreator   func(config map[string]interface{}) (SamplerStrategy, error)
	FormatterCreator func(config map[string]interface{}) (FormatterStrategy, error)
)

// Configuration Types

// LoggerConfig for logger setup
type LoggerConfig struct {
	Level        domain.Level
	ServiceName  string
	Environment  string
	Sinks        []Sink
	Filters      []FilterStrategy
	Sampler      SamplerStrategy
	Formatter    FormatterStrategy
	BufferSize   int
	AsyncLogging bool
	Features     []string
	CustomConfig map[string]interface{}
}

// SinkConfig for sink setup
type SinkConfig struct {
	Type     string
	Settings map[string]interface{}
}

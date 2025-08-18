package outbound

import (
	"context"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

// Sink is an outbound port for writing logs to external systems
type Sink interface {
	Write(entry *domain.LogEntry) error
	Close() error
	Name() string
	Health() error
}

// ConfigSrc is an outbound port for fetching configuration from external sources
type ConfigSrc interface {
	// Get log level for specific module and tenant
	Level(module, tenant string) domain.Level

	// Get filter rules
	Rules() []domain.FilterRule

	// Get sampling rate
	Sample(module string) float32

	// Subscribe to configuration changes
	Sub(callback func()) error

	// Health check
	Health() error

	// Close connection
	Close() error
}

// Metrics is an outbound port for recording metrics to external systems
type Metrics interface {
	// Record log metrics
	Log(ctx context.Context, level domain.Level, module string, latency time.Duration) error

	// Record error metrics
	Err(ctx context.Context, module string, errorType string) error

	// Record custom metric
	Rec(ctx context.Context, name string, value float64, tags map[string]string) error

	// Get current metrics
	Get() map[string]interface{}

	// Health check
	Health() error

	// Close connection
	Close() error
}

// Alerter is an outbound port for sending alerts to external systems
type Alerter interface {
	// Send alert
	Send(ctx context.Context, alert Alert) error

	// Check if alert should be sent (rate limiting, deduplication)
	ShouldSend(ctx context.Context, alert Alert) bool

	// Health check
	Health() error

	// Close connection
	Close() error
}

// Alert represents an alert to be sent
type Alert struct {
	Title string
	Msg   string
	Level domain.Level
	Cat   string
	Data  map[string]interface{}
	Time  time.Time
	Src   string
	Tags  []string
}

// TraceSrc is an outbound port for distributed tracing
type TraceSrc interface {
	// Extract trace context
	TraceID(ctx context.Context) string
	SpanID(ctx context.Context) string

	// Create new span
	StartSpan(ctx context.Context, name string) (context.Context, Span)

	// Get tracer
	Tracer(name string) Tracer

	// Health check
	Health() error

	// Close connection
	Close() error
}

// Span represents a trace span
type Span interface {
	// Set attributes
	Attr(key string, value interface{})
	Attrs(attrs map[string]interface{})

	// Record error
	Err(err error)

	// Set status
	Status(code SpanStatusCode, description string)

	// End span
	End()
}

// SpanStatusCode represents span status
type SpanStatusCode int

const (
	SpanStatusUnset SpanStatusCode = iota
	SpanStatusOK
	SpanStatusError
)

// Tracer creates spans
type Tracer interface {
	Start(ctx context.Context, spanName string) (context.Context, Span)
}

// Storage Ports for persistence

type LogStorage interface {
	Store(ctx context.Context, entry *domain.LogEntry) error
	Query(ctx context.Context, query LogQuery) ([]domain.LogEntry, error)
	DelOld(ctx context.Context, before time.Time) error
	Health() error
	Close() error
}

type LogQuery struct {
	Start  time.Time
	End    time.Time
	Level  *domain.Level
	Mod    string
	Tenant string
	UID    string
	TID    string
	RID    string
	Search string
	Limit  int
	Offset int
}

// Strategy Ports

type Filter interface {
	Apply(entry *domain.LogEntry) *domain.LogEntry
	Name() string
	Configure(config map[string]interface{}) error
}

type Sampler interface {
	Sample(entry *domain.LogEntry) bool
	Name() string
	Configure(config map[string]interface{}) error
}

type Formatter interface {
	Format(entry *domain.LogEntry) string
	Name() string
	Configure(config map[string]interface{}) error
}

// Factory Ports

type Factory interface {
	Filter(name string, config map[string]interface{}) (Filter, error)
	Sampler(name string, config map[string]interface{}) (Sampler, error)
	Formatter(name string, config map[string]interface{}) (Formatter, error)
	RegFilter(name string, creator FilterCreator)
	RegSampler(name string, creator SamplerCreator)
	RegFormatter(name string, creator FormatterCreator)
}

type (
	FilterCreator    func(config map[string]interface{}) (Filter, error)
	SamplerCreator   func(config map[string]interface{}) (Sampler, error)
	FormatterCreator func(config map[string]interface{}) (Formatter, error)
)

type SinkFactory interface {
	Create(config SinkConfig) (Sink, error)
	Reg(name string, creator SinkCreator)
	List() []string
}

type SinkCreator func(config map[string]interface{}) (Sink, error)

type SinkConfig struct {
	Type string
	Cfg  map[string]interface{}
}

type HTTPClient interface {
	Post(ctx context.Context, url string, headers map[string]string, body []byte) (*HTTPResponse, error)
	Get(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error)
}

type HTTPResponse struct {
	Code int
	Hdrs map[string]string
	Body []byte
}

type Queue interface {
	Pub(ctx context.Context, queue string, msg []byte, headers map[string]string) error
	Sub(ctx context.Context, queue string, handler func(msg []byte, headers map[string]string)) error
	Close() error
}

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	Close() error
}

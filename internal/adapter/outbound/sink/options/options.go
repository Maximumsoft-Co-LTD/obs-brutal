// Package options provides options-based configuration for constructing
// loggers and sinks via the facade without relying on environment variables.
package options

import (
    "fmt"
    "obs-brutal/internal/core/domain"
    "obs-brutal/internal/core/port"
    secsvc "obs-brutal/internal/core/service/security"
    "reflect"
)

// ===== CONFIGURATION OPTIONS =====

// ConfigOptions holds high-level configuration for logger construction
// and observability integrations.
type ConfigOptions struct {
	ServiceName        string
	Version            string
	Environment        string
	MetricEndpoint     string
	OTelEndpoint       string
	PromtailEndpoint   string
	PrometheusEndpoint string
	LokiEndpoint       string
	LokiLabels         map[string]string
	OTelResource       map[string]string
	MaskingEnabled     bool
	AsyncEnabled       bool
    LogLevel           domain.Level
	UseZerolog         bool
}

// NewConfigOptions creates configuration with sensible defaults.
func NewConfigOptions() *ConfigOptions {
    return &ConfigOptions{
        ServiceName:    "obs-brutal-service",
        Version:        "1.0.0",
        Environment:    "production",
        LogLevel:       domain.InfoLevel,
        MaskingEnabled: false, // Only when explicitly enabled
        AsyncEnabled:   false, // Only when explicitly enabled
        LokiLabels:     map[string]string{},
        OTelResource:   map[string]string{},
    }
}

// ===== OPTION BUILDERS =====

// ConfigOption represents a configuration option used by New/SmartConfig.
type ConfigOption func(*ConfigOptions)

// SrvName sets service name
func SrvName(name string) ConfigOption { return func(opts *ConfigOptions) { opts.ServiceName = name } }

// Version sets service version
func Version(version string) ConfigOption {
	return func(opts *ConfigOptions) { opts.Version = version }
}

// Env sets environment
func Env(environment string) ConfigOption {
	return func(opts *ConfigOptions) { opts.Environment = environment }
}

// MetricEndpoint sets metric endpoint
func MetricEndpoint(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) { opts.MetricEndpoint = endpoint }
}

// OTel sets OTEL endpoint
func OTel(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) { opts.OTelEndpoint = endpoint }
}

// Promtail sets Promtail endpoint
func Promtail(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) { opts.PromtailEndpoint = endpoint }
}

// Prometheus sets Prometheus endpoint
func Prometheus(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) { opts.PrometheusEndpoint = endpoint }
}

// Loki sets Loki endpoint
func Loki(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) { opts.LokiEndpoint = endpoint }
}

// LokiLabels sets default labels for Loki/Promtail
func LokiLabels(labels map[string]string) ConfigOption {
	return func(opts *ConfigOptions) { opts.LokiLabels = labels }
}

// Masking enables PII masking
func Masking(enabled bool) ConfigOption {
	return func(opts *ConfigOptions) { opts.MaskingEnabled = enabled }
}

// Async enables async pipeline
func Async(enabled bool) ConfigOption {
	return func(opts *ConfigOptions) { opts.AsyncEnabled = enabled }
}

// LogLevel sets log level
func LogLevel(level domain.Level) ConfigOption { return func(opts *ConfigOptions) { opts.LogLevel = level } }

// Zerolog enables zerolog sink usage
func Zerolog(enabled bool) ConfigOption {
	return func(opts *ConfigOptions) { opts.UseZerolog = enabled }
}

// OTelResource sets resource attributes for OTLP export
func OTelResource(attrs map[string]string) ConfigOption {
	return func(opts *ConfigOptions) { opts.OTelResource = attrs }
}

// ===== SMART CONFIGURATION =====

// SmartConfig provides intelligent configuration derived from options.
type SmartConfig struct{ options *ConfigOptions }

// NewSmartConfig creates smart configuration with options.
func NewSmartConfig(opts ...ConfigOption) *SmartConfig {
	config := NewConfigOptions()
	for _, opt := range opts {
		opt(config)
	}
	return &SmartConfig{options: config}
}

// HasOTEL reports whether an OTEL endpoint is configured.
func (sc *SmartConfig) HasOTEL() bool { return sc.options.OTelEndpoint != "" }

// HasSecurity reports whether security features should be enabled.
func (sc *SmartConfig) HasSecurity() bool { return sc.options.MaskingEnabled }

// HasAsync reports whether async mode should be enabled.
func (sc *SmartConfig) HasAsync() bool { return sc.options.AsyncEnabled }

// HasMetrics reports whether metrics endpoints are configured.
func (sc *SmartConfig) HasMetrics() bool {
	return sc.options.MetricEndpoint != "" || sc.options.PrometheusEndpoint != ""
}

// GetServiceName returns service name
func (sc *SmartConfig) GetServiceName() string { return sc.options.ServiceName }

// GetVersion returns version
func (sc *SmartConfig) GetVersion() string { return sc.options.Version }

// GetEnvironment returns environment
func (sc *SmartConfig) GetEnvironment() string { return sc.options.Environment }

// GetOTELEndpoint returns OTEL endpoint
func (sc *SmartConfig) GetOTELEndpoint() string { return sc.options.OTelEndpoint }

// GetLogLevel returns configured log level
func (sc *SmartConfig) GetLogLevel() domain.Level { return sc.options.LogLevel }

// ===== RESPONSE OPTIONS =====

// ResponseOpts provides fluent response building for web handlers.
type ResponseOpts struct{}

// NewResponseOpts creates response options helper.
func NewResponseOpts() *ResponseOpts { return &ResponseOpts{} }

// Msg sets message
func (r *ResponseOpts) Msg(msg string) ResponseOption {
	return func(rb port.ResponseBuilder) { rb.Msg(msg) }
}

// Body sets response body
func (r *ResponseOpts) Body(data interface{}) ResponseOption {
	return func(rb port.ResponseBuilder) { rb.Body(data) }
}

// Status sets status message
func (r *ResponseOpts) Status(status string) ResponseOption {
	return func(rb port.ResponseBuilder) { rb.Status(status) }
}

// Detail sets error detail
func (r *ResponseOpts) Detail(detail string) ResponseOption {
	return func(rb port.ResponseBuilder) { rb.Detail(detail) }
}

// Prt enables response printing
func (r *ResponseOpts) Prt(enabled bool) ResponseOption {
	return func(rb port.ResponseBuilder) { rb.Prt(enabled) }
}

// ResponseOption configures response builder
type ResponseOption func(port.ResponseBuilder)

// ===== EXTRACT UTILITIES =====

// ExtractOptions provides field extraction utilities
type ExtractOptions struct{}

// Fields extracts and masks struct fields in one line
func (e *ExtractOptions) Fields(data interface{}) map[string]interface{} {
    masker := secsvc.NewPIIMasker()
    fields := extractStructFieldsAdvanced(data)
    return masker.MaskFields(fields)
}

// extractStructFieldsAdvanced extracts fields with advanced patterns
func extractStructFieldsAdvanced(data interface{}) map[string]interface{} {
	if data == nil {
		return make(map[string]interface{})
	}
	fields := make(map[string]interface{})
	switch v := data.(type) {
	case map[string]interface{}:
		return v
	default:
		if isStruct(v) {
			return extractFromStruct(v)
		}
		fields["data"] = fmt.Sprintf("%+v", v)
	}
	return fields
}

// isStruct checks if value is a struct
func isStruct(v interface{}) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	return rv.IsValid() && rv.Kind() == reflect.Struct
}

// extractFromStruct extracts fields from struct
func extractFromStruct(v interface{}) map[string]interface{} {
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(v)
	if rt.Kind() == reflect.Ptr {
		rv = rv.Elem()
		rt = rt.Elem()
	}
	if !rv.IsValid() || rt.Kind() != reflect.Struct {
		return map[string]interface{}{"data": fmt.Sprintf("%+v", v)}
	}
	out := make(map[string]interface{}, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			tok := jsonTag
			if idx := indexComma(tok); idx >= 0 {
				tok = tok[:idx]
			}
			if tok != "" && tok != "-" {
				name = tok
			}
		}
		val := rv.Field(i).Interface()
		out[name] = val
	}
	return out
}

// indexComma returns index of first comma or -1
func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

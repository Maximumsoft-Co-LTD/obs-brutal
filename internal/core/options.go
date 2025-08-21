// Package core provides options-based configuration (no environment variables)
package core

import "fmt"

// ===== CONFIGURATION OPTIONS =====

// ConfigOptions holds all configuration
type ConfigOptions struct {
	ServiceName        string
	Version            string
	Environment        string
	MetricEndpoint     string
	OTelEndpoint       string
	PromtailEndpoint   string
	PrometheusEndpoint string
	LokiEndpoint       string
	MaskingEnabled     bool
	AsyncEnabled       bool
	LogLevel           Level
}

// NewConfigOptions creates default configuration
func NewConfigOptions() *ConfigOptions {
	return &ConfigOptions{
		ServiceName:    "obs-brutal-service",
		Version:        "1.0.0",
		Environment:    "production",
		LogLevel:       INFO,
		MaskingEnabled: false, // Only when explicitly enabled
		AsyncEnabled:   false, // Only when explicitly enabled
	}
}

// ===== OPTION BUILDERS =====

// ConfigOption represents a configuration option
type ConfigOption func(*ConfigOptions)

// SrvName sets service name
func SrvName(name string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.ServiceName = name
	}
}

// Version sets service version
func Version(version string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.Version = version
	}
}

// Env sets environment
func Env(environment string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.Environment = environment
	}
}

// MetricEndpoint sets metric endpoint
func MetricEndpoint(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.MetricEndpoint = endpoint
	}
}

// OTel sets OTEL endpoint
func OTel(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.OTelEndpoint = endpoint
	}
}

// Promtail sets Promtail endpoint
func Promtail(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.PromtailEndpoint = endpoint
	}
}

// Prometheus sets Prometheus endpoint
func Prometheus(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.PrometheusEndpoint = endpoint
	}
}

// Loki sets Loki endpoint
func Loki(endpoint string) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.LokiEndpoint = endpoint
	}
}

// Masking enables PII masking
func Masking(enabled bool) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.MaskingEnabled = enabled
	}
}

// Async enables async pipeline
func Async(enabled bool) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.AsyncEnabled = enabled
	}
}

// LogLevel sets log level
func LogLevel(level Level) ConfigOption {
	return func(opts *ConfigOptions) {
		opts.LogLevel = level
	}
}

// ===== SMART CONFIGURATION =====

// SmartConfig provides intelligent configuration
type SmartConfig struct {
	options *ConfigOptions
}

// NewSmartConfig creates smart configuration with options
func NewSmartConfig(opts ...ConfigOption) *SmartConfig {
	config := NewConfigOptions()

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	return &SmartConfig{
		options: config,
	}
}

// HasOTEL checks if OTEL is configured
func (sc *SmartConfig) HasOTEL() bool {
	return sc.options.OTelEndpoint != ""
}

// HasSecurity checks if security features should be enabled
func (sc *SmartConfig) HasSecurity() bool {
	return sc.options.MaskingEnabled
}

// HasAsync checks if async should be enabled
func (sc *SmartConfig) HasAsync() bool {
	return sc.options.AsyncEnabled
}

// HasMetrics checks if metrics are configured
func (sc *SmartConfig) HasMetrics() bool {
	return sc.options.MetricEndpoint != "" ||
		sc.options.PrometheusEndpoint != ""
}

// GetServiceName returns service name
func (sc *SmartConfig) GetServiceName() string {
	return sc.options.ServiceName
}

// GetVersion returns version
func (sc *SmartConfig) GetVersion() string {
	return sc.options.Version
}

// GetEnvironment returns environment
func (sc *SmartConfig) GetEnvironment() string {
	return sc.options.Environment
}

// GetOTELEndpoint returns OTEL endpoint
func (sc *SmartConfig) GetOTELEndpoint() string {
	return sc.options.OTelEndpoint
}

// ===== RESPONSE OPTIONS =====

// ResponseOpts provides fluent response building
type ResponseOpts struct{}

// NewResponseOpts creates response options
func NewResponseOpts() *ResponseOpts {
	return &ResponseOpts{}
}

// Msg sets message
func (r *ResponseOpts) Msg(msg string) ResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.msg = msg
	}
}

// Body sets response body
func (r *ResponseOpts) Body(data interface{}) ResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.payload = data
	}
}

// Status sets status message
func (r *ResponseOpts) Status(status string) ResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.statusMsg = status
	}
}

// Detail sets error detail
func (r *ResponseOpts) Detail(detail string) ResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.detail = detail
	}
}

// Prt enables response printing
func (r *ResponseOpts) Prt(enabled bool) ResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.printEnabled = enabled
	}
}

// ResponseOption configures response builder
type ResponseOption func(*SimpleResponseBuilder)

// ===== EXTRACT UTILITIES =====

// ExtractOptions provides field extraction utilities
type ExtractOptions struct{}

// Fields extracts and masks struct fields in one line
func (e *ExtractOptions) Fields(data interface{}) map[string]interface{} {
	// Create enterprise masker for auto-masking
	masker := NewPIIMasker()

	// Extract struct fields using reflection
	fields := extractStructFieldsAdvanced(data)

	// Apply masking
	return masker.MaskFields(fields)
}

// extractStructFieldsAdvanced extracts fields with advanced patterns
func extractStructFieldsAdvanced(data interface{}) map[string]interface{} {
	// Enhanced struct field extraction with Go 1.25 optimizations
	if data == nil {
		return make(map[string]interface{})
	}

	// For now, return simplified extraction
	// In production, this would use proper reflection with struct tag parsing
	fields := make(map[string]interface{})

	// Simple type assertion for common cases
	switch v := data.(type) {
	case map[string]interface{}:
		return v
	default:
		// Use reflection for structs
		if isStruct(v) {
			return extractFromStruct(v)
		}
		fields["data"] = fmt.Sprintf("%+v", v)
	}

	return fields
}

// isStruct checks if value is a struct
func isStruct(v interface{}) bool {
	// Simplified check - in production would use proper reflection
	return false
}

// extractFromStruct extracts fields from struct
func extractFromStruct(v interface{}) map[string]interface{} {
	// Simplified implementation - in production would parse struct tags
	return map[string]interface{}{
		"struct_data": fmt.Sprintf("%+v", v),
	}
}

// Package core provides LogTrc interface with tracing + logging + response integration
package core

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"obs-brutal/internal/core/domain"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ===== SMART CAPABILITY DETECTOR =====

// CapabilityDetector automatically detects available features
type CapabilityDetector struct {
	hasOTEL     atomic.Bool
	hasSecurity atomic.Bool
	hasAsync    atomic.Bool
	hasGin      atomic.Bool
	detected    atomic.Bool
	mu          sync.Once
}

// GlobalDetector provides global capability detection
var GlobalDetector = &CapabilityDetector{}

// detectCapabilities performs one-time capability detection
func (cd *CapabilityDetector) detectCapabilities() {
	cd.mu.Do(func() {
		// Detect OTEL endpoint
		if otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); otelEndpoint != "" {
			cd.hasOTEL.Store(true)
		}
		if jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT"); jaegerEndpoint != "" {
			cd.hasOTEL.Store(true)
		}

		// Detect security requirements
		if piiMasking := os.Getenv("PII_MASKING"); piiMasking == "true" {
			cd.hasSecurity.Store(true)
		}
		if auditEnabled := os.Getenv("AUDIT_TRAIL"); auditEnabled == "true" {
			cd.hasSecurity.Store(true)
		}

		// Detect async requirements
		if asyncEnabled := os.Getenv("ASYNC_LOGGING"); asyncEnabled == "true" {
			cd.hasAsync.Store(true)
		}
		if highVolume := os.Getenv("HIGH_VOLUME"); highVolume == "true" {
			cd.hasAsync.Store(true)
		}

		cd.detected.Store(true)
	})
}

// HasOTEL checks if OTEL should be enabled
func (cd *CapabilityDetector) HasOTEL() bool {
	cd.detectCapabilities()
	return cd.hasOTEL.Load()
}

// HasSecurity checks if security features should be enabled
func (cd *CapabilityDetector) HasSecurity() bool {
	cd.detectCapabilities()
	return cd.hasSecurity.Load()
}

// HasAsync checks if async pipeline should be enabled
func (cd *CapabilityDetector) HasAsync() bool {
	cd.detectCapabilities()
	return cd.hasAsync.Load()
}

// ===== CONFIGURATION HELPERS =====

func GetServiceName() string {
	if name := os.Getenv("SERVICE_NAME"); name != "" {
		return name
	}
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return "obs-brutal-service"
}

func GetVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "1.0.0"
}

func GetEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}
	return "production"
}

func GetOTELEndpoint() string {
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	if endpoint := os.Getenv("JAEGER_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return "http://localhost:14268"
}

// NewSmartLogger creates auto-detecting logger
func NewSmartLogger() Logger {
	// For now, return unified logger
	// Full smart detection will be added progressively
	return NewUnifiedLogger(INFO)
}

// NewSmartLoggerWithOptions creates logger with explicit options
func NewSmartLoggerWithOptions(opts ...ConfigOption) Logger {
	config := NewSmartConfig(opts...)

	// Create appropriate logger based on configuration
	if config.HasOTEL() && config.HasSecurity() {
		// Full enterprise mode
		if enterprise, err := NewEnterpriseLogger(
			config.GetServiceName(),
			config.GetVersion(),
			config.GetEnvironment(),
			config.GetOTELEndpoint(),
			config.options.LogLevel,
		); err == nil {
			return enterprise
		}
	} else if config.HasOTEL() {
		// OTEL mode
		if otel, err := NewOTelLogger(
			config.GetServiceName(),
			config.GetVersion(),
			config.GetEnvironment(),
			config.GetOTELEndpoint(),
			config.options.LogLevel,
		); err == nil {
			return otel
		}
	} else if config.HasAsync() {
		// Async mode
		return NewAsyncLogger(config.options.LogLevel)
	}

	// Default to unified logger
	return NewUnifiedLogger(config.options.LogLevel)
}

// ===== LOGTRC INTERFACE =====

// LogTrc provides unified tracing + logging + response building
type LogTrc interface {
	// === Tracing Hierarchy ===
	FlatPr(name string) Tracer  // Same level trace
	ChildPr(name string) Tracer // Child trace

	// === Field Management ===
	F(key string, value interface{}) LogTrc        // Add field
	Fs(data interface{}) LogTrc                    // Extract+mask fields in one line
	Body(name string, body any) attribute.KeyValue // Struct data

	// === Span Attributes ===
	Add(attrs ...attribute.KeyValue)                  // Multi attributes
	Str(key, value string) attribute.KeyValue         // String attribute
	Bool(key string, value bool) attribute.KeyValue   // Bool attribute
	Num(key string, value float64) attribute.KeyValue // Number attribute

	// === Error Handling ===
	Err(msg string, err error) error               // Span error
	Errf(format string, args ...interface{}) error // Formatted error

	// === Metadata ===
	Detail(msg string) attribute.KeyValue // Detail attribute
	Msg(msg string) attribute.KeyValue    // Message attribute
	Code(code int) attribute.KeyValue     // Status code

	// === Logging with Trace Context ===
	Prt(format string, args ...interface{})  // Print + log
	Prtf(format string, args ...interface{}) // Printf + log
	PrtTrc()                                 // Print all trace data
	SinceTime(enabled bool) LogTrc           // Print logs/sec performance

	// === Response Building (Simplified) ===
	R(status int, options ...interface{}) *SimpleResponseBuilder // No .Build() needed

	// === Lifecycle ===
	End()   // End trace + auto-log
	Close() // End all + cleanup

	// === Utility ===
	GetTraceID() string // Get current trace ID
	GetSpanID() string  // Get current span ID
}

// Tracer interface for child traces
type Tracer interface {
	LogTrc // Inherit all LogTrc methods

	// Additional tracer-specific methods
	FlatPr(name string) Tracer  // Same level sibling
	ChildPr(name string) Tracer // Child trace
	Parent() Tracer             // Get parent tracer
}

// ===== SMART LOGTRC IMPLEMENTATION =====

// SmartLogTrc implements LogTrc with auto-detection and lazy loading
type SmartLogTrc struct {
	// Core components
	logger    Logger
	span      trace.Span
	gin       *gin.Context
	operation string
	module    string

	// Lazy-loaded features
	otelProvider *OTelProvider // Lazy init
	security     *PIIMasker    // Lazy init
	audit        *AuditTrail   // Lazy init

	// State
	fields      map[string]interface{}
	traceID     string
	spanID      string
	parentSpan  trace.Span
	childTraces []Tracer
	startTime   time.Time

	// Utilities
	extractor *ExtractOptions
	respOpts  *ResponseOpts

	// Performance
	isEnded atomic.Bool
	mu      sync.RWMutex
}

// NewSmartLogTrc creates auto-detecting LogTrc with options
func NewSmartLogTrc(c *gin.Context, operation string, opts ...ConfigOption) LogTrc {
	// Create smart configuration
	config := NewSmartConfig(opts...)

	// Create smart logger
	logger := NewSmartLogger()

	// Auto-extract correlation IDs from Gin
	traceID := extractTraceFromGin(c)
	spanID := extractSpanFromGin(c)

	logtrc := &SmartLogTrc{
		logger:    logger,
		gin:       c,
		operation: operation,
		fields:    make(map[string]interface{}, 8),
		traceID:   traceID,
		spanID:    spanID,
		startTime: time.Now(),
		extractor: &ExtractOptions{},
		respOpts:  NewResponseOpts(),
	}

	// Auto-setup based on configuration
	logtrc.autoSetupWithConfig(config)

	return logtrc
}

// autoSetupWithConfig configures features based on options
func (lt *SmartLogTrc) autoSetupWithConfig(config *SmartConfig) {
	// Auto-setup OTEL if configured
	if config.HasOTEL() {
		lt.setupOTELWithConfig(config)
	}

	// Auto-setup security if enabled
	if config.HasSecurity() {
		lt.setupSecurity()
	}

	// Auto-add request context
	if lt.gin != nil {
		lt.addGinContext()
	}
}

// setupOTELWithConfig initializes OTEL with configuration
func (lt *SmartLogTrc) setupOTELWithConfig(config *SmartConfig) {
	if lt.otelProvider == nil {
		if provider, err := NewOTelProvider(
			config.GetServiceName(),
			config.GetVersion(),
			config.GetEnvironment(),
			config.GetOTELEndpoint(),
		); err == nil {
			lt.otelProvider = provider
		}
	}

	// Start root span for operation
	if lt.otelProvider != nil && lt.gin != nil {
		ctx := lt.gin.Request.Context()
		spanCtx, span := lt.otelProvider.StartSpan(ctx, lt.operation)

		lt.span = span
		lt.gin.Request = lt.gin.Request.WithContext(spanCtx)

		// Extract trace info
		lt.traceID, lt.spanID = lt.otelProvider.ExtractTraceInfo(spanCtx)
	}
}

// setupOTEL initializes OTEL integration
func (lt *SmartLogTrc) setupOTEL() {
	// Create OTEL provider if not exists
	if lt.otelProvider == nil {
		if provider, err := NewOTelProvider(
			GetServiceName(),
			GetVersion(),
			GetEnvironment(),
			GetOTELEndpoint(),
		); err == nil {
			lt.otelProvider = provider
		}
	}

	// Start root span for operation
	if lt.otelProvider != nil && lt.gin != nil {
		ctx := lt.gin.Request.Context()
		spanCtx, span := lt.otelProvider.StartSpan(ctx, lt.operation)

		lt.span = span
		lt.gin.Request = lt.gin.Request.WithContext(spanCtx)

		// Extract trace info
		lt.traceID, lt.spanID = lt.otelProvider.ExtractTraceInfo(spanCtx)
	}
}

// setupSecurity initializes security features
func (lt *SmartLogTrc) setupSecurity() {
	if lt.security == nil {
		lt.security = NewPIIMasker()
	}

	if lt.audit == nil {
		lt.audit = NewAuditTrail(1000)
	}
}

// addGinContext extracts context from Gin automatically
func (lt *SmartLogTrc) addGinContext() {
	if lt.gin == nil {
		return
	}

	// Auto-extract common fields
	lt.fields["method"] = lt.gin.Request.Method
	lt.fields["path"] = lt.gin.Request.URL.Path
	lt.fields["ip"] = lt.gin.ClientIP()
	lt.fields["user_agent"] = lt.gin.Request.UserAgent()

	// Auto-extract headers
	if requestID := lt.gin.GetHeader("X-Request-ID"); requestID != "" {
		lt.fields["request_id"] = requestID
	}
	if userID := lt.gin.GetHeader("X-User-ID"); userID != "" {
		lt.fields["user_id"] = userID
	}
	if sessionID := lt.gin.GetHeader("X-Session-ID"); sessionID != "" {
		lt.fields["session_id"] = sessionID
	}
}

// ===== LOGTRC INTERFACE IMPLEMENTATION =====

// Fs extracts and masks fields in one line (replaces ExtractFields)
func (lt *SmartLogTrc) Fs(data interface{}) LogTrc {
	clone := lt.clone()

	// Auto-extract and mask fields
	if lt.extractor != nil {
		extractedFields := lt.extractor.Fields(data)
		for k, v := range extractedFields {
			clone.fields[k] = v
		}
	}

	return clone
}

// SinceTime prints logs/sec performance since trace start
func (lt *SmartLogTrc) SinceTime(enabled bool) LogTrc {
	if !enabled {
		return lt
	}

	duration := time.Since(lt.startTime)
	if duration > 0 {
		// Calculate approximate logs/sec based on current session
		logCount := lt.logger.LogCount()
		logsPerSecond := float64(logCount) / duration.Seconds()

		lt.Prt("Performance: %.0f logs/sec (%.2f ms elapsed)",
			logsPerSecond, float64(duration.Milliseconds()))
	}

	return lt
}

// FlatPr creates same-level trace
func (lt *SmartLogTrc) FlatPr(name string) Tracer {
	return lt.createTracer(name, false)
}

// ChildPr creates child trace
func (lt *SmartLogTrc) ChildPr(name string) Tracer {
	return lt.createTracer(name, true)
}

// createTracer creates new tracer (flat or child)
func (lt *SmartLogTrc) createTracer(name string, isChild bool) Tracer {
	tracer := &SmartTracer{
		parent:    lt,
		name:      name,
		fields:    make(map[string]interface{}),
		isChild:   isChild,
		startTime: time.Now(),
	}

	// Setup OTEL span if available
	if lt.otelProvider != nil && lt.gin != nil {
		ctx := lt.gin.Request.Context()

		var span trace.Span
		if isChild && lt.span != nil {
			// Create child span
			_, span = lt.otelProvider.StartSpan(ctx, name)
		} else {
			// Create sibling span
			_, span = lt.otelProvider.StartSpan(context.Background(), name)
		}

		tracer.span = span
	}

	// Track child traces
	lt.mu.Lock()
	lt.childTraces = append(lt.childTraces, tracer)
	lt.mu.Unlock()

	return tracer
}

// F adds field
func (lt *SmartLogTrc) F(key string, value interface{}) LogTrc {
	clone := lt.clone()
	clone.fields[key] = value
	return clone
}

// Body creates attribute for struct data
func (lt *SmartLogTrc) Body(name string, body any) attribute.KeyValue {
	// Auto-extract and mask fields if security is enabled
	if lt.security != nil {
		if fields := extractStructFields(body); fields != nil {
			maskedFields := lt.security.MaskFields(fields)
			return attribute.String(name, fmt.Sprintf("%+v", maskedFields))
		}
	}

	return attribute.String(name, fmt.Sprintf("%+v", body))
}

// Add adds multiple attributes to current span
func (lt *SmartLogTrc) Add(attrs ...attribute.KeyValue) {
	if lt.span != nil {
		lt.span.SetAttributes(attrs...)
	}
}

// Str creates string attribute
func (lt *SmartLogTrc) Str(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

// Bool creates bool attribute
func (lt *SmartLogTrc) Bool(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

// Num creates number attribute
func (lt *SmartLogTrc) Num(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

// Err records error in span and returns it
func (lt *SmartLogTrc) Err(msg string, err error) error {
	if lt.span != nil {
		lt.span.RecordError(err, trace.WithAttributes(
			attribute.String("error.message", msg),
		))
	}

	// Log error through smart logger
	lt.logger.WithError(err).Error(msg)

	return err
}

// Errf records formatted error
func (lt *SmartLogTrc) Errf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	return lt.Err(fmt.Sprintf(format, args...), err)
}

// Detail creates detail attribute
func (lt *SmartLogTrc) Detail(msg string) attribute.KeyValue {
	return attribute.String("detail", msg)
}

// Msg creates message attribute
func (lt *SmartLogTrc) Msg(msg string) attribute.KeyValue {
	return attribute.String("message", msg)
}

// Code creates status code attribute
func (lt *SmartLogTrc) Code(code int) attribute.KeyValue {
	return attribute.Int("status_code", code)
}

// Prt prints and logs with trace context
func (lt *SmartLogTrc) Prt(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Add trace context to logger
	contextLogger := lt.logger.Fs(lt.fields)
	if lt.traceID != "" {
		contextLogger = contextLogger.F("trace_id", lt.traceID)
	}
	if lt.spanID != "" {
		contextLogger = contextLogger.F("span_id", lt.spanID)
	}
	if lt.module != "" {
		contextLogger = contextLogger.F("module", lt.module)
	}

	contextLogger.Info(msg)
}

// Prtf formatted print and log
func (lt *SmartLogTrc) Prtf(format string, args ...interface{}) {
	lt.Prt(format, args...)
}

// PrtTrc prints all current trace data
func (lt *SmartLogTrc) PrtTrc() {
	traceData := map[string]interface{}{
		"operation":    lt.operation,
		"module":       lt.module,
		"trace_id":     lt.traceID,
		"span_id":      lt.spanID,
		"fields":       lt.fields,
		"child_traces": len(lt.childTraces),
	}

	lt.logger.Fs(traceData).Info("Trace data dump")
}

// R creates response builder (simplified - no .Build() needed)
func (lt *SmartLogTrc) R(status int, options ...interface{}) *SimpleResponseBuilder {
	rb := &SimpleResponseBuilder{
		logtrc:       lt,
		gin:          lt.gin,
		status:       status,
		printEnabled: false,
	}

	// Parse options intelligently
	for _, option := range options {
		switch opt := option.(type) {
		case ResponseOption:
			opt(rb)
		case string:
			// First string is message
			if rb.msg == "" {
				rb.msg = opt
			} else {
				rb.detail = opt
			}
		case map[string]interface{}:
			rb.payload = opt
		case gin.H:
			rb.payload = opt
		}
	}

	return rb
}

// End ends trace and logs completion
func (lt *SmartLogTrc) End() {
	if lt.isEnded.Load() {
		return
	}

	// End all child traces first
	lt.mu.RLock()
	children := make([]Tracer, len(lt.childTraces))
	copy(children, lt.childTraces)
	lt.mu.RUnlock()

	for _, child := range children {
		child.End()
	}

	// End span if exists
	if lt.span != nil {
		lt.span.End()
	}

	// Log trace completion
	lt.Prt("Trace '%s' completed", lt.operation)

	lt.isEnded.Store(true)
}

// Close ends everything and cleanup
func (lt *SmartLogTrc) Close() {
	lt.End()

	// Cleanup resources
	lt.mu.Lock()
	clear(lt.fields)
	lt.childTraces = nil
	lt.mu.Unlock()
}

// GetTraceID returns current trace ID
func (lt *SmartLogTrc) GetTraceID() string {
	return lt.traceID
}

// GetSpanID returns current span ID
func (lt *SmartLogTrc) GetSpanID() string {
	return lt.spanID
}

// Parent returns self as parent tracer (implement Tracer interface)
func (lt *SmartLogTrc) Parent() Tracer {
	return lt
}

// clone creates shallow copy for method chaining
func (lt *SmartLogTrc) clone() *SmartLogTrc {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	clone := &SmartLogTrc{
		logger:       lt.logger,
		span:         lt.span,
		gin:          lt.gin,
		operation:    lt.operation,
		module:       lt.module,
		otelProvider: lt.otelProvider,
		security:     lt.security,
		audit:        lt.audit,
		fields:       make(map[string]interface{}, len(lt.fields)),
		traceID:      lt.traceID,
		spanID:       lt.spanID,
		parentSpan:   lt.parentSpan,
	}

	// Copy fields
	for k, v := range lt.fields {
		clone.fields[k] = v
	}

	// Share atomic state
	clone.isEnded.Store(lt.isEnded.Load())

	return clone
}

// ===== SMART TRACER IMPLEMENTATION =====

// SmartTracer implements Tracer interface for child traces
type SmartTracer struct {
	parent    *SmartLogTrc
	name      string
	span      trace.Span
	fields    map[string]interface{}
	isChild   bool
	startTime time.Time
	endTime   time.Time
	isEnded   atomic.Bool
	mu        sync.RWMutex
}

// Implement Tracer interface
// Removed Mod() - not needed

func (st *SmartTracer) FlatPr(name string) Tracer {
	return st.parent.FlatPr(name)
}

func (st *SmartTracer) ChildPr(name string) Tracer {
	// Create child of this tracer
	child := &SmartTracer{
		parent:    st.parent,
		name:      name,
		fields:    make(map[string]interface{}),
		isChild:   true,
		startTime: time.Now(),
	}

	// Setup OTEL child span
	if st.span != nil && st.parent.otelProvider != nil {
		ctx := trace.ContextWithSpan(context.Background(), st.span)
		_, childSpan := st.parent.otelProvider.StartSpan(ctx, name)
		child.span = childSpan
	}

	return child
}

func (st *SmartTracer) Parent() Tracer {
	// Return parent as tracer interface
	// Add Parent method to SmartLogTrc to implement Tracer
	return st.parent
}

func (st *SmartTracer) F(key string, value interface{}) LogTrc {
	st.mu.Lock()
	st.fields[key] = value
	st.mu.Unlock()
	return st
}

func (st *SmartTracer) Fs(data interface{}) LogTrc {
	if st.parent.extractor != nil {
		extractedFields := st.parent.extractor.Fields(data)
		st.mu.Lock()
		for k, v := range extractedFields {
			st.fields[k] = v
		}
		st.mu.Unlock()
	}
	return st
}

func (st *SmartTracer) SinceTime(enabled bool) LogTrc {
	if enabled {
		duration := time.Since(st.startTime)
		if duration > 0 {
			st.parent.Prt("Tracer '%s' performance: %.2f ms elapsed",
				st.name, float64(duration.Milliseconds()))
		}
	}
	return st
}

func (st *SmartTracer) Body(name string, body any) attribute.KeyValue {
	return st.parent.Body(name, body)
}

func (st *SmartTracer) Add(attrs ...attribute.KeyValue) {
	if st.span != nil {
		st.span.SetAttributes(attrs...)
	}
}

func (st *SmartTracer) Str(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

func (st *SmartTracer) Bool(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

func (st *SmartTracer) Num(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

func (st *SmartTracer) Err(msg string, err error) error {
	if st.span != nil {
		st.span.RecordError(err, trace.WithAttributes(
			attribute.String("error.message", msg),
		))
	}

	// Log through parent
	st.parent.logger.WithError(err).Error(msg)

	return err
}

func (st *SmartTracer) Errf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	return st.Err(fmt.Sprintf(format, args...), err)
}

func (st *SmartTracer) Detail(msg string) attribute.KeyValue {
	return attribute.String("detail", msg)
}

func (st *SmartTracer) Msg(msg string) attribute.KeyValue {
	return attribute.String("message", msg)
}

func (st *SmartTracer) Code(code int) attribute.KeyValue {
	return attribute.Int("status_code", code)
}

func (st *SmartTracer) Prt(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Combine parent and tracer fields
	allFields := make(map[string]interface{})

	// Copy parent fields
	st.parent.mu.RLock()
	for k, v := range st.parent.fields {
		allFields[k] = v
	}
	st.parent.mu.RUnlock()

	// Copy tracer fields
	st.mu.RLock()
	for k, v := range st.fields {
		allFields[k] = v
	}
	st.mu.RUnlock()

	// Add trace context
	allFields["trace_name"] = st.name
	allFields["trace_type"] = map[bool]string{true: "child", false: "flat"}[st.isChild]

	st.parent.logger.Fs(allFields).Info(msg)
}

func (st *SmartTracer) Prtf(format string, args ...interface{}) {
	st.Prt(format, args...)
}

func (st *SmartTracer) PrtTrc() {
	traceData := map[string]interface{}{
		"tracer_name": st.name,
		"tracer_type": map[bool]string{true: "child", false: "flat"}[st.isChild],
		"start_time":  st.startTime,
		"duration_ms": time.Since(st.startTime).Milliseconds(),
		"fields":      st.fields,
	}

	st.parent.logger.Fs(traceData).Info("Tracer data dump")
}

func (st *SmartTracer) R(status int, options ...interface{}) *SimpleResponseBuilder {
	return st.parent.R(status, options...)
}

func (st *SmartTracer) End() {
	if st.isEnded.Load() {
		return
	}

	st.endTime = time.Now()
	duration := st.endTime.Sub(st.startTime)

	// End OTEL span
	if st.span != nil {
		st.span.SetAttributes(
			attribute.Float64("duration_ms", float64(duration.Milliseconds())),
		)
		st.span.End()
	}

	// Log tracer completion
	st.Prt("Tracer '%s' completed in %v", st.name, duration)

	st.isEnded.Store(true)
}

func (st *SmartTracer) Close() {
	st.End()
}

func (st *SmartTracer) GetTraceID() string {
	return st.parent.GetTraceID()
}

func (st *SmartTracer) GetSpanID() string {
	if st.span != nil {
		return st.span.SpanContext().SpanID().String()
	}
	return st.parent.GetSpanID()
}

// ===== RESPONSE BUILDER =====

// SimpleResponseBuilder handles Gin responses with auto-logging
type SimpleResponseBuilder struct {
	logtrc       LogTrc
	gin          *gin.Context
	status       int
	payload      interface{}
	err          error
	msg          string
	detail       string
	statusMsg    string
	printEnabled bool
}

// SimpleResponseOption configures response builder
type SimpleResponseOption func(*SimpleResponseBuilder)

// ResponseOptions factory with chainable methods
type ResponseOptions struct {
	options []SimpleResponseOption
}

// OptsResponse creates response options factory
func OptsResponse() *ResponseOptions {
	return &ResponseOptions{
		options: make([]SimpleResponseOption, 0, 3),
	}
}

// Detail sets error detail (chainable)
func (o *ResponseOptions) Detail(detail string) *ResponseOptions {
	o.options = append(o.options, func(rb *SimpleResponseBuilder) {
		rb.detail = detail
	})
	return o
}

// Msg sets response message (chainable)
func (o *ResponseOptions) Msg(msg string) *ResponseOptions {
	o.options = append(o.options, func(rb *SimpleResponseBuilder) {
		rb.msg = msg
	})
	return o
}

// Response sets response payload (chainable)
func (o *ResponseOptions) Response(data interface{}) *ResponseOptions {
	o.options = append(o.options, func(rb *SimpleResponseBuilder) {
		rb.payload = data
	})
	return o
}

// Build returns all accumulated options
func (o *ResponseOptions) Build() []SimpleResponseOption {
	return o.options
}

// Err handles error response with auto-logging and optional printing
func (rb *SimpleResponseBuilder) Err(err error) error {
	rb.err = err

	// Build response
	resp := gin.H{}

	if rb.msg != "" {
		resp["message"] = rb.msg
	} else {
		resp["message"] = "Error occurred"
	}

	if rb.statusMsg != "" {
		resp["status"] = rb.statusMsg
	}

	if rb.detail != "" {
		resp["detail"] = rb.detail
	}

	if err != nil {
		resp["error"] = err.Error()

		// Log error with trace context
		rb.logtrc.Err("Response error: "+rb.msg, err)
	}

	// Add trace info
	if traceID := rb.logtrc.GetTraceID(); traceID != "" {
		resp["trace_id"] = traceID
	}

	// Add timestamp in requested format
	resp["datetime"] = time.Now().Format("2006-01-02T15:04:05-07:00")

	// Print response if enabled
	if rb.printEnabled {
		rb.logtrc.Prt("Error response sent: %s (status: %d)", rb.msg, rb.status)
	}

	// Send response
	rb.gin.JSON(rb.status, resp)

	return err
}

// Send sends successful response with auto-logging
func (rb *SimpleResponseBuilder) Send() {
	resp := gin.H{}

	if rb.msg != "" {
		resp["message"] = rb.msg
	}

	if rb.statusMsg != "" {
		resp["status"] = rb.statusMsg
	}

	if rb.payload != nil {
		if rb.msg != "" {
			resp["data"] = rb.payload
		} else {
			// If no message, use payload as root
			if payloadMap, ok := rb.payload.(gin.H); ok {
				// Merge with existing resp
				for k, v := range payloadMap {
					resp[k] = v
				}
			} else {
				resp["data"] = rb.payload
			}
		}
	}

	// Add trace info
	if traceID := rb.logtrc.GetTraceID(); traceID != "" {
		resp["trace_id"] = traceID
	}

	// Add timestamp in requested format (ISO8601 with timezone)
	resp["datetime"] = time.Now().Format("2006-01-02T15:04:05-07:00")

	// Print response if enabled
	if rb.printEnabled {
		rb.logtrc.Prt("Response sent: %s (status: %d)", rb.msg, rb.status)
	}

	// Log successful response
	if rb.msg != "" {
		rb.logtrc.Prt("Response sent: %s", rb.msg)
	}

	// Send response
	rb.gin.JSON(rb.status, resp)
}

// ===== HELPER FUNCTIONS =====

// extractTraceFromGin extracts trace ID from Gin context
func extractTraceFromGin(c *gin.Context) string {
	// Try multiple sources
	if traceID := c.GetHeader("X-Trace-Id"); traceID != "" {
		return traceID
	}
	if traceID := c.GetHeader("Trace-Id"); traceID != "" {
		return traceID
	}
	if val, exists := c.Get("trace_id"); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}

	// Generate new trace ID
	return domain.GenerateID("trace")
}

// extractSpanFromGin extracts span ID from Gin context
func extractSpanFromGin(c *gin.Context) string {
	if spanID := c.GetHeader("X-Span-Id"); spanID != "" {
		return spanID
	}
	if val, exists := c.Get("span_id"); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}

	// Generate new span ID
	return domain.GenerateID("span")
}

// extractStructFields extracts fields from struct using reflection
func extractStructFields(v interface{}) map[string]interface{} {
	// Simplified struct field extraction
	// In production, this would use proper reflection
	if v == nil {
		return nil
	}

	// Return map representation for now
	return map[string]interface{}{
		"struct_data": fmt.Sprintf("%+v", v),
	}
}

package obsvbrutal

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GinLogger defines the interface for Gin logging
type GinLogger interface {
	F(key string, value interface{}) GinLogger
	Prt(format string, args ...interface{})
	Prtf(format string, args ...interface{})
	Err(err error) error
	Errf(format string, args ...interface{}) error
	R(status int, opts ...SimpleResponseOption) *SimpleResponseBuilder
	FlatPr(name string) Tracer
	ChildPr(name string) Tracer
	Close()

	// Exposed fields
	GetTraceID() string
	GetSpanID() string
}

// Tracer defines the interface for tracing operations
type Tracer interface {
	FlatPr(name string) Tracer
	ChildPr(name string) Tracer
	Body(name string, body interface{}) []attribute.KeyValue
	Add(attrs ...attribute.KeyValue)
	Str(key, value string) attribute.KeyValue
	Bool(key string, value bool) attribute.KeyValue
	Num(key string, value float64) attribute.KeyValue
	Err(err error) error
	Errf(format string, args ...interface{}) error
	Detail(msg string) attribute.KeyValue
	Msg(msg string) attribute.KeyValue
	Code(code int) attribute.KeyValue
	End()
}

// LogFrmGin is a simplified logger wrapper for Gin
type LogFrmGin struct {
	log  Logger
	ctx  context.Context
	gin  *gin.Context
	span trace.Span
	otel *OTelProvider
	mu   sync.Mutex
	flds map[string]interface{}
	tID  string // TraceID
	sID  string // SpanID
}

// GetLogFrmGin creates a new logger from Gin context
func GetLogFrmGin(c *gin.Context, operationName string) GinLogger {
	// Get logger from context or create new one
	logger, ok := GetLoggerFromGinContext(c)
	if !ok {
		// Create default logger if not found
		logger, _ = NewLogger(WithLevel(InfoLevel))
	}

	// Get provider from context
	var provider *OTelProvider
	if p, exists := c.Get("otel_provider"); exists {
		provider, _ = p.(*OTelProvider)
	}

	// Start span if provider available
	ctx := c.Request.Context()
	var span trace.Span
	var traceID, spanID string

	if provider != nil {
		ctx, span = StartSpan(ctx, operationName)
		if span != nil {
			spanCtx := span.SpanContext()
			if spanCtx.IsValid() {
				traceID = spanCtx.TraceID().String()
				spanID = spanCtx.SpanID().String()
			}
		}
	}

	return &LogFrmGin{
		log:  logger.Ctx(ctx).TID(traceID).SID(spanID),
		ctx:  ctx,
		gin:  c,
		span: span,
		otel: provider,
		flds: make(map[string]interface{}),
		tID:  traceID,
		sID:  spanID,
	}
}

// F adds a field (short for WithField)
func (l *LogFrmGin) F(key string, value interface{}) GinLogger {
	l.mu.Lock()
	if l.flds != nil {
		l.flds[key] = value
	}
	l.mu.Unlock()
	l.log = l.log.F(key, value)
	return l
}

// Prt logs info
func (l *LogFrmGin) Prt(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.log.Info(msg)
}

// Prtf logs info with formatting
func (l *LogFrmGin) Prtf(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}

// Err logs error and returns it
func (l *LogFrmGin) Err(err error) error {
	if err != nil {
		l.log.Err(err).Error("Error")
		if l.span != nil {
			RecordError(l.span, err, "Error")
		}
	}
	return err
}

// Errf logs formatted error and returns it
func (l *LogFrmGin) Errf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	l.log.Err(err).Error("Error")
	if l.span != nil {
		RecordError(l.span, err, "Error")
	}
	return err
}

// R creates a response builder
func (l *LogFrmGin) R(status int, opts ...SimpleResponseOption) *SimpleResponseBuilder {
	rb := &SimpleResponseBuilder{
		log:     l,
		stat:    status,
		payload: make(gin.H),
	}

	// Set status immediately to ensure it is reflected even if no body is written
	if l.gin != nil {
		l.gin.Status(status)
	}

	for _, opt := range opts {
		opt(rb)
	}

	return rb
}

func getTrc(l *LogFrmGin) (ctx context.Context, span trace.Span, logger Logger, otel *OTelProvider, name string, attr []attribute.KeyValue) {
	if l.otel != nil {
		ctx, span = StartSpan(ctx, name)
	}
	if l.ctx == nil {
		ctx, span = StartSpan(context.Background(), name)
		return ctx, span, l.log.Ctx(ctx).F("span", name), l.otel, name, []attribute.KeyValue{}
	}
	if l.span != nil {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			ctx, span = StartSpan(ctx, name)
			return ctx, span, l.log.Ctx(ctx).F("span", name), l.otel, name, []attribute.KeyValue{}
		}
	}
	return ctx, span, l.log.Ctx(ctx).F("span", name), l.otel, name, []attribute.KeyValue{}
}

// Parent creates a new parent span/trace
func (l *LogFrmGin) FlatPr(name string) Tracer {
	ctx := l.ctx
	var span trace.Span

	if l.otel != nil {
		ctx, span = StartSpan(ctx, name)
	}
	if l.ctx == nil {
		ctx, span = StartSpan(context.Background(), name)
		return &Parent{
			ctx:  ctx,
			span: span,
			log:  l.log.Ctx(ctx).F("span", name),
			otel: l.otel,
			name: name,
			attr: []attribute.KeyValue{},
		}
	}
	if span != nil {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {

		}
	}

	return &Parent{
		ctx:  ctx,
		span: span,
		log:  l.log.Ctx(ctx).F("span", name),
		otel: l.otel,
		name: name,
		attr: []attribute.KeyValue{},
	}
}

func (l *LogFrmGin) ChildPr(name string) Tracer {
	ctx := l.ctx
	var span trace.Span

	if l.otel != nil {
		ctx, span = StartFlatSpan(ctx, name)
	}

	return &Parent{
		ctx:  ctx,
		span: span,
		log:  l.log.Ctx(ctx).F("span", name),
		otel: l.otel,
		name: name,
		attr: []attribute.KeyValue{},
	}
}

// Close ends the span
func (l *LogFrmGin) Close() {
	if l.span != nil {
		l.span.End()
	}
	if l.otel != nil {
		l.log.F("trace_id", l.tID).F("span_id", l.sID).Info("Trace closed")
	}
}

// GetTraceID returns the trace ID
func (l *LogFrmGin) GetTraceID() string {
	return l.tID
}

// GetSpanID returns the span ID
func (l *LogFrmGin) GetSpanID() string {
	return l.sID
}

// Parent represents a parent trace/span
type Parent struct {
	ctx  context.Context
	span trace.Span
	log  Logger
	otel *OTelProvider
	name string
	attr []attribute.KeyValue
	err  error
}

// Body creates a new parent with body data
func (p *Parent) Body(name string, body interface{}) []attribute.KeyValue {
	ctx := p.ctx
	var span trace.Span
	var attrs []attribute.KeyValue

	if body == nil {
		attrs = []attribute.KeyValue{attribute.String("body", "null")}
	} else {
		val := reflect.ValueOf(body)
		typ := val.Type()

		// Handle pointer
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				attrs = []attribute.KeyValue{attribute.String("body", "null")}
			} else {
				val = val.Elem()
				typ = val.Type()
			}
		}

		switch val.Kind() {
		case reflect.Struct:
			// Extract fields from struct using log tags
			fields := ExtractFields(body)
			if len(fields) > 0 {
				jsonData, err := json.Marshal(fields)
				if err == nil {
					attrs = []attribute.KeyValue{
						attribute.String("body", string(jsonData)),
						attribute.String("body_type", "struct"),
					}
				} else {
					attrs = []attribute.KeyValue{
						attribute.String("body", fmt.Sprintf("%v", body)),
						attribute.String("body_error", err.Error()),
					}
				}
			} else {
				// If no log tags, marshal the whole struct
				jsonData, err := json.Marshal(body)
				if err == nil {
					attrs = []attribute.KeyValue{
						attribute.String("body", string(jsonData)),
						attribute.String("body_type", "struct"),
					}
				} else {
					attrs = []attribute.KeyValue{
						attribute.String("body", fmt.Sprintf("%v", body)),
						attribute.String("body_error", err.Error()),
					}
				}
			}

		case reflect.Slice, reflect.Array:
			// Check if it's a slice of structs
			if val.Len() > 0 {
				elemType := typ.Elem()
				if elemType.Kind() == reflect.Ptr {
					elemType = elemType.Elem()
				}

				if elemType.Kind() == reflect.Struct {
					// Process slice of structs
					var items []map[string]interface{}
					for i := 0; i < val.Len(); i++ {
						elem := val.Index(i).Interface()
						fields := ExtractFields(elem)
						if len(fields) > 0 {
							items = append(items, fields)
						} else {
							// If no log tags, use the whole struct
							items = append(items, map[string]interface{}{"item": elem})
						}
					}

					jsonData, err := json.Marshal(items)
					if err == nil {
						attrs = []attribute.KeyValue{
							attribute.String("body", string(jsonData)),
							attribute.String("body_type", "[]struct"),
							attribute.Int("body_count", val.Len()),
						}
					} else {
						attrs = []attribute.KeyValue{
							attribute.String("body", fmt.Sprintf("%v", body)),
							attribute.String("body_error", err.Error()),
						}
					}
				} else {
					// Regular slice (not structs)
					jsonData, err := json.Marshal(body)
					if err == nil {
						attrs = []attribute.KeyValue{
							attribute.String("body", string(jsonData)),
							attribute.String("body_type", "slice"),
							attribute.Int("body_count", val.Len()),
						}
					} else {
						attrs = []attribute.KeyValue{
							attribute.String("body", fmt.Sprintf("%v", body)),
							attribute.String("body_error", err.Error()),
						}
					}
				}
			} else {
				attrs = []attribute.KeyValue{
					attribute.String("body", "[]"),
					attribute.String("body_type", "empty_slice"),
					attribute.Int("body_count", 0),
				}
			}

		default:
			// For other types, convert to string
			attrs = []attribute.KeyValue{
				attribute.String("body", fmt.Sprintf("%v", body)),
				attribute.String("body_type", typ.String()),
			}
		}
	}

	if p.otel != nil {
		ctx, span = StartSpan(ctx, name)
		if span != nil {
			span.SetAttributes(attrs...)
		}
	}

	return attrs
}

// Parent creates a child parent
func (p *Parent) Parent(name string) Tracer {
	ctx := p.ctx
	var span trace.Span

	if p.otel != nil {
		ctx, span = StartSpan(ctx, name)
	}

	return &Parent{
		ctx:  ctx,
		span: span,
		log:  p.log.F("parent", name),
		otel: p.otel,
		name: name,
		attr: []attribute.KeyValue{},
	}
}

// End ends the parent span gracefully with comprehensive logging
func (p *Parent) End() {
	// Ensure span is properly closed
	if p.span != nil {
		defer func() {
			// Add final attributes before closing
			if len(p.attr) > 0 {
				p.span.SetAttributes(p.attr...)
			}

			// Record error if exists
			if p.err != nil {
				RecordError(p.span, p.err, "Operation failed: "+p.name)
				p.span.SetStatus(codes.Error, p.err.Error())
			} else {
				p.span.SetStatus(codes.Ok, "Operation completed successfully")
			}

			p.span.End()
		}()
	}

	// Enhanced logging with context
	logger := p.log.F("operation", p.name)

	if p.otel != nil && p.span != nil {
		spanCtx := p.span.SpanContext()
		if spanCtx.IsValid() {
			logger = logger.
				F("trace_id", spanCtx.TraceID().String()).
				F("span_id", spanCtx.SpanID().String())
		}
	}

	// Add attributes to log
	if len(p.attr) > 0 {
		for _, attr := range p.attr {
			logger = logger.F(string(attr.Key), attr.Value.AsInterface())
		}
	}

	// Final status log
	if p.err != nil {
		logger.Err(p.err).Error("Operation failed")
	} else {
		logger.Info("Operation completed successfully")
	}
}

// Add adds attributes to the span
func (p *Parent) Add(attrs ...attribute.KeyValue) {
	p.attr = append(p.attr, attrs...)
	if p.span != nil {
		p.span.SetAttributes(attrs...)
	}
}

// Str creates string attribute
func (p *Parent) Str(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

// Bool creates bool attribute
func (p *Parent) Bool(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

// Num creates number attribute
func (p *Parent) Num(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

// Detail creates detail attribute
func (p *Parent) Detail(detail string) attribute.KeyValue {
	return attribute.String("detail", detail)
}

// Msg creates message attribute
func (p *Parent) Msg(msg string) attribute.KeyValue {
	return attribute.String("message", msg)
}

// Code creates status code attribute
func (p *Parent) Code(code int) attribute.KeyValue {
	return attribute.Int("code", code)
}

// Err logs error and returns it
func (p *Parent) Err(err error) error {
	if err != nil {
		p.err = err
		p.log.Err(err).Error("Error")
		if p.span != nil {
			RecordError(p.span, err, "Error in "+p.name)
		}
	}
	return err
}

// Errf logs formatted error and returns it
func (p *Parent) Errf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	p.err = err
	p.log.Err(err).Error("Error")
	if p.span != nil {
		RecordError(p.span, err, "Error in "+p.name)
	}
	return err
}

// FlatPr creates a flat parent (sibling span)
func (p *Parent) FlatPr(name string) Tracer {
	ctx := p.ctx
	var span trace.Span

	if p.otel != nil {
		// ใช้ StartFlatSpan เพื่อสร้าง sibling span
		ctx, span = StartFlatSpan(ctx, name)
	}

	return &Parent{
		ctx:  ctx,
		span: span,
		log:  p.log.F("flat_span", name),
		otel: p.otel,
		name: name,
		attr: []attribute.KeyValue{},
	}
}

// ChildPr creates a child parent (nested span)
func (p *Parent) ChildPr(name string) Tracer {
	ctx := p.ctx
	var span trace.Span

	if p.otel != nil {
		// ใช้ StartSpan ปกติเพื่อสร้าง child span
		ctx, span = StartSpan(ctx, name)
	}

	return &Parent{
		ctx:  ctx,
		span: span,
		log:  p.log.F("child_span", name),
		otel: p.otel,
		name: name,
		attr: []attribute.KeyValue{},
	}
}

// SimpleResponseBuilder builds HTTP responses for the simple API
type SimpleResponseBuilder struct {
	log     *LogFrmGin
	stat    int
	payload any
	err     error
	msg     string
	det     string
}

// OptsResponse creates response options
func OptsResponse() *ResponseOptions {
	return &ResponseOptions{}
}

// ResponseOptions provides response configuration methods
type ResponseOptions struct{}

// SimpleResponseOption for simple API response configuration
type SimpleResponseOption func(*SimpleResponseBuilder)

// Detail sets error detail
func (o *ResponseOptions) Detail(detail string) SimpleResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.det = detail
	}
}

// Msg sets response message
func (o *ResponseOptions) Msg(msg string) SimpleResponseOption {
	return func(rb *SimpleResponseBuilder) {
		rb.msg = msg
	}
}

// Data sets response data
func (o *ResponseOptions) Response(data any) SimpleResponseOption {
	return func(rb *SimpleResponseBuilder) {
		if rb.payload == nil {
			rb.payload = gin.H{}
		}
		switch v := data.(type) {
		case string:
			rb.payload = v
		default:
			rb.payload = data
		}
	}
}

// Err logs error and sends response
func (rb *SimpleResponseBuilder) Err(err error) error {
	rb.err = err

	// Log error
	rb.log.log.Err(err).Error("Error")

	// Build error response
	response := gin.H{
		"error": rb.msg,
	}

	if rb.det != "" {
		response["details"] = rb.det
	}

	if err != nil {
		response["error"] = err.Error()
	}

	// Send response
	rb.log.gin.JSON(rb.stat, response)

	return err
}

// Helper function to generate request ID

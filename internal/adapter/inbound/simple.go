package inbound

import (
	"context"
	"fmt"

	pin "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

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
	GetTraceID() string
	GetSpanID() string
}

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

type LogFrmGin struct {
	log pin.Logger
	ctx context.Context
	gin *gin.Context
	pID string
	tID string
	sID string
}

func getLoggerFromGin(c *gin.Context) (pin.Logger, bool) {
	if v, ok := c.Get("logger"); ok {
		if l, ok2 := v.(pin.Logger); ok2 {
			return l, true
		}
	}
	return nil, false
}

func GetLogFrmGin(c *gin.Context, operationName string) GinLogger {
	logger, ok := getLoggerFromGin(c)
	if !ok {
		l, _ := NewZapLoggerAdapter()
		logger = l
	}
	ctx := c.Request.Context()
	return &LogFrmGin{log: logger.Ctx(ctx), ctx: ctx, gin: c}
}

func (l *LogFrmGin) F(key string, value interface{}) GinLogger { l.log = l.log.F(key, value); return l }
func (l *LogFrmGin) Prt(format string, args ...interface{})    { l.log.Info(fmt.Sprintf(format, args...)) }
func (l *LogFrmGin) Prtf(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}
func (l *LogFrmGin) Err(err error) error {
	if err != nil {
		l.log.Err(err).Error("Error")
	}
	return err
}
func (l *LogFrmGin) Errf(format string, args ...interface{}) error {
	return l.Err(fmt.Errorf(format, args...))
}
func (l *LogFrmGin) Close()             {}
func (l *LogFrmGin) GetTraceID() string { return l.tID }
func (l *LogFrmGin) GetSpanID() string  { return l.sID }

func (l *LogFrmGin) R(status int, opts ...SimpleResponseOption) *SimpleResponseBuilder {
	rb := &SimpleResponseBuilder{log: l, stat: status, payload: gin.H{}}
	if l.gin != nil {
		l.gin.Status(status)
	}
	for _, opt := range opts {
		opt(rb)
	}
	return rb
}

type Parent struct {
	ctx  context.Context
	log  pin.Logger
	name string
	attr []attribute.KeyValue
	err  error
}

func (l *LogFrmGin) FlatPr(name string) Tracer {
	return &Parent{ctx: l.ctx, log: l.log.F("span", name), name: name}
}
func (l *LogFrmGin) ChildPr(name string) Tracer {
	return &Parent{ctx: l.ctx, log: l.log.F("span", name), name: name}
}

func (p *Parent) FlatPr(name string) Tracer {
	return &Parent{ctx: p.ctx, log: p.log.F("span", name), name: name}
}
func (p *Parent) ChildPr(name string) Tracer {
	return &Parent{ctx: p.ctx, log: p.log.F("span", name), name: name}
}

// Parent compatibility method used by examples
func (p *Parent) Parent(name string) Tracer { return p.ChildPr(name) }

func (p *Parent) Body(name string, body interface{}) []attribute.KeyValue { return nil }
func (p *Parent) Add(attrs ...attribute.KeyValue)                         { p.attr = append(p.attr, attrs...) }
func (p *Parent) Str(key, value string) attribute.KeyValue                { return attribute.String(key, value) }
func (p *Parent) Bool(key string, value bool) attribute.KeyValue          { return attribute.Bool(key, value) }
func (p *Parent) Num(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}
func (p *Parent) Detail(detail string) attribute.KeyValue { return attribute.String("detail", detail) }
func (p *Parent) Msg(msg string) attribute.KeyValue       { return attribute.String("message", msg) }
func (p *Parent) Code(code int) attribute.KeyValue        { return attribute.Int("code", code) }
func (p *Parent) Err(err error) error {
	if err != nil {
		p.err = err
		p.log.Err(err).Error("Error")
	}
	return err
}
func (p *Parent) Errf(format string, args ...interface{}) error {
	return p.Err(fmt.Errorf(format, args...))
}
func (p *Parent) End() {
	if p.err != nil {
		p.log.Err(p.err).Error("Operation failed")
	} else {
		p.log.Info("Operation completed successfully")
	}
}

// Response builder

type SimpleResponseBuilder struct {
	log     *LogFrmGin
	stat    int
	payload any
	err     error
	msg     string
	det     string
}

type ResponseOptions struct{}

type SimpleResponseOption func(*SimpleResponseBuilder)

func OptsResponse() *ResponseOptions { return &ResponseOptions{} }
func (o *ResponseOptions) Detail(detail string) SimpleResponseOption {
	return func(rb *SimpleResponseBuilder) { rb.det = detail }
}
func (o *ResponseOptions) Msg(msg string) SimpleResponseOption {
	return func(rb *SimpleResponseBuilder) { rb.msg = msg }
}
func (o *ResponseOptions) Response(data any) SimpleResponseOption {
	return func(rb *SimpleResponseBuilder) { rb.payload = data }
}

func (rb *SimpleResponseBuilder) Err(err error) error {
	rb.err = err
	rb.log.log.Err(err).Error("Error")
	resp := gin.H{"error": rb.msg}
	if rb.det != "" {
		resp["details"] = rb.det
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	rb.log.gin.JSON(rb.stat, resp)
	return err
}

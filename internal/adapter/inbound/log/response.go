package log

import (
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"

	"github.com/gin-gonic/gin"
)

// SimpleResponseBuilder is a minimal response builder for Gin that
// formats a JSON response and optionally prints a short log line
// through a tiny logger interface (Prt/GetTraceID).
type SimpleResponseBuilder struct {
	logtrc interface {
		Prt(format string, args ...interface{})
		GetTraceID() string
	}
	gin          *gin.Context
	status       int
	payload      interface{}
	err          error
	msg          string
	detail       string
	statusMsg    string
	printEnabled bool
}

// NewSimpleResponseBuilder constructs a response builder bound to the Gin
// context and an internal printer (logger) that provides Prt/GetTraceID.
// It returns a port.ResponseBuilder so callers in higher layers depend on
// the port, not the concrete implementation.
func NewSimpleResponseBuilder(g *gin.Context, l interface {
	Prt(format string, args ...interface{})
	GetTraceID() string
}, status int) port.ResponseBuilder {
	return &SimpleResponseBuilder{gin: g, logtrc: l, status: status}
}

// Msg sets the top-level message string.
func (rb *SimpleResponseBuilder) Msg(m string) { rb.msg = m }

// Body sets the response payload (placed under data if message is present).
func (rb *SimpleResponseBuilder) Body(v interface{}) { rb.payload = v }

// Status sets a status string (separate from HTTP status code).
func (rb *SimpleResponseBuilder) Status(s string) { rb.statusMsg = s }

// Detail sets an optional error detail string.
func (rb *SimpleResponseBuilder) Detail(d string) { rb.detail = d }

// Prt enables printing a short log line via the provided printer.
func (rb *SimpleResponseBuilder) Prt(enabled bool) { rb.printEnabled = enabled }

// build constructs the base JSON payload with consistent fields.
func (rb *SimpleResponseBuilder) build() gin.H {
	resp := gin.H{}
	if rb.msg != "" {
		resp["message"] = rb.msg
	}
	if rb.statusMsg != "" {
		resp["status"] = rb.statusMsg
	}
	if tid := rb.logtrc.GetTraceID(); tid != "" {
		resp["trace_id"] = tid
	}
	resp["datetime"] = time.Now().Format(time.RFC3339Nano)
	return resp
}

// Err writes an error JSON response using the previously configured fields
// (message/status/detail) and returns the error for caller chaining.
func (rb *SimpleResponseBuilder) Err(err error) error {
	rb.err = err
	resp := rb.build()
	if rb.msg == "" {
		resp["message"] = "Error occurred"
	}
	if rb.detail != "" {
		resp["detail"] = rb.detail
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	if rb.printEnabled {
		rb.logtrc.Prt("resp err: %d", rb.status)
	}
	rb.gin.JSON(rb.status, resp)
	return err
}

// Send writes a success JSON response using the configured fields and payload.
func (rb *SimpleResponseBuilder) Send() {
	resp := rb.build()
	if rb.payload != nil {
		if rb.msg != "" {
			resp["data"] = rb.payload
		} else if m, ok := rb.payload.(gin.H); ok {
			for k, v := range m {
				resp[k] = v
			}
		} else {
			resp["data"] = rb.payload
		}
	}
	if rb.printEnabled {
		rb.logtrc.Prt("resp ok: %d", rb.status)
	}
	rb.gin.JSON(rb.status, resp)
}

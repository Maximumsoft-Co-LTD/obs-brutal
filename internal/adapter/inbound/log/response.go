package log

import (
    "time"

    "obs-brutal/internal/core/port"

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
func (rb *SimpleResponseBuilder) Msg(m string)       { rb.msg = m }
// Body sets the response payload (placed under data if message is present).
func (rb *SimpleResponseBuilder) Body(v interface{}) { rb.payload = v }
// Status sets a status string (separate from HTTP status code).
func (rb *SimpleResponseBuilder) Status(s string)    { rb.statusMsg = s }
// Detail sets an optional error detail string.
func (rb *SimpleResponseBuilder) Detail(d string)    { rb.detail = d }
// Prt enables printing a short log line via the provided printer.
func (rb *SimpleResponseBuilder) Prt(enabled bool)   { rb.printEnabled = enabled }

// Err writes an error JSON response using the previously configured fields
// (message/status/detail) and returns the error for caller chaining.
func (rb *SimpleResponseBuilder) Err(err error) error {
    rb.err = err
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
		rb.logtrc.Prt("Response error: %s", rb.msg)
	}
	if tid := rb.logtrc.GetTraceID(); tid != "" {
		resp["trace_id"] = tid
	}
	resp["datetime"] = time.Now().Format("2006-01-02T15:04:05-07:00")
	if rb.printEnabled {
		rb.logtrc.Prt("Error response sent: %s (status: %d)", rb.msg, rb.status)
	}
    rb.gin.JSON(rb.status, resp)
    return err
}

// Send writes a success JSON response using the configured fields and payload.
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
		} else if m, ok := rb.payload.(gin.H); ok {
			for k, v := range m {
				resp[k] = v
			}
		} else {
			resp["data"] = rb.payload
		}
	}
	if tid := rb.logtrc.GetTraceID(); tid != "" {
		resp["trace_id"] = tid
	}
	resp["datetime"] = time.Now().Format("2006-01-02T15:04:05-07:00")
	if rb.printEnabled {
		rb.logtrc.Prt("Response sent: %s (status: %d)", rb.msg, rb.status)
	}
	if rb.msg != "" {
		rb.logtrc.Prt("Response sent: %s", rb.msg)
	}
	rb.gin.JSON(rb.status, resp)
}

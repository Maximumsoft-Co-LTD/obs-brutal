package inbound

import (
	"bytes"
	"fmt"
	"io"
	"obs-brutal/internal/core/port/inbound"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// MiddlewareAdapter provides middleware for HTTP frameworks
type MiddlewareAdapter struct {
	logger inbound.Logger
}

// NewMiddlewareAdapter creates a new middleware adapter
func NewMiddlewareAdapter(logger inbound.Logger) *MiddlewareAdapter {
	return &MiddlewareAdapter{
		logger: logger,
	}
}

// GinLoggingMiddleware creates a Gin middleware for logging
func (a *MiddlewareAdapter) GinLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip health checks
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		// Create logger with request context
		logger := a.logger.Ctx(c.Request.Context())

		// Add request information
		logger = logger.
			F("method", c.Request.Method).
			F("path", c.Request.URL.Path).
			F("client_ip", c.ClientIP()).
			F("user_agent", c.Request.UserAgent())

		// Extract request ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		logger = logger.RID(requestID)

		// Extract user information if available
		if userID := c.GetString("user_id"); userID != "" {
			logger = logger.UID(userID)
		}
		if sessionID := c.GetString("session_id"); sessionID != "" {
			logger = logger.Sess(sessionID)
		}
		if tenantID := c.GetString("tenant_id"); tenantID != "" {
			logger = logger.Tenant(tenantID)
		}

		// Store logger in context
		c.Set("logger", logger)
		c.Set("request_id", requestID)

		// Capture request body if needed
		var reqBody []byte
		if c.Request.Body != nil && shouldLogBody(c.Request.Method, c.Request.URL.Path) {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Create response writer wrapper
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = rw

		// Log request
		logger.F("query", c.Request.URL.RawQuery).
			Info("Request received")

		// Process request
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Log response
		status := c.Writer.Status()
		logger = logger.
			F("status", status).
			F("duration_ms", duration.Milliseconds()).
			F("response_size", rw.body.Len())

		// Log based on status
		switch {
		case status >= 500:
			if len(reqBody) > 0 {
				logger = logger.F("request_body", string(reqBody))
			}
			logger = logger.F("response_body", truncateBody(rw.body.Bytes()))
			logger.Error("Server error")
		case status >= 400:
			if len(reqBody) > 0 {
				logger = logger.F("request_body", string(reqBody))
			}
			logger = logger.F("response_body", truncateBody(rw.body.Bytes()))
			logger.Warn("Client error")
		case duration > 1*time.Second:
			logger.Warn("Slow request")
		default:
			logger.Info("Request completed")
		}
	}
}

// GinTracingMiddleware creates a Gin middleware for tracing
func (a *MiddlewareAdapter) GinTracingMiddleware(tracer interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// This would integrate with OpenTelemetry or other tracing systems
		// For now, just pass through
		c.Next()
	}
}

// GinErrorHandlingMiddleware creates a Gin middleware for error handling
func (a *MiddlewareAdapter) GinErrorHandlingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get logger from context
				logger := a.getLoggerFromContext(c)

				// Log panic
				logger.F("panic", err).
					F("stack", getStackTrace()).
					Error("Panic recovered")

				// Return error response
				c.JSON(500, gin.H{
					"error":      "Internal server error",
					"request_id": c.GetString("request_id"),
				})
				c.Abort()
			}
		}()

		c.Next()

		// Handle errors from context
		if len(c.Errors) > 0 {
			logger := a.getLoggerFromContext(c)

			for _, err := range c.Errors {
				logger.Err(err.Err).
					F("type", err.Type).
					Error("Request error")
			}
		}
	}
}

// responseWriter wraps gin.ResponseWriter to capture response
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Helper functions

func (a *MiddlewareAdapter) getLoggerFromContext(c *gin.Context) inbound.Logger {
	if logger, ok := c.Get("logger"); ok {
		if l, ok := logger.(inbound.Logger); ok {
			return l
		}
	}
	return a.logger
}

func shouldLogBody(method, path string) bool {
	// Don't log body for GET requests or file uploads
	if method == "GET" || method == "HEAD" {
		return false
	}

	// Don't log file upload paths
	if contains(path, "/upload") || contains(path, "/file") {
		return false
	}

	return true
}

func truncateBody(body []byte) string {
	const maxLen = 1000
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "...(truncated)"
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

func getStackTrace() string {
	buf := make([]byte, 1024*8)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

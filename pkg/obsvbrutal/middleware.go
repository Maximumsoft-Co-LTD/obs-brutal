package obsvbrutal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware creates HTTP middleware for logging
func HTTPMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Extract trace context
			ctx := r.Context()

			// Create request ID if not exists
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = GenerateRequestID()
			}
			ctx = context.WithValue(ctx, "request_id", requestID)

			// Extract user info from headers/context
			userID := r.Header.Get("X-User-ID")
			if userID != "" {
				ctx = context.WithValue(ctx, "user_id", userID)
			}

			// Extract tenant info
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID != "" {
				ctx = context.WithValue(ctx, "tenant_id", tenantID)
			}

			// Create logger with context
			reqLogger := logger.
				Ctx(ctx).
				RID(requestID).
				IP(getClientIP(r)).
				F("method", r.Method).
				F("path", r.URL.Path).
				F("user_agent", r.UserAgent())

			if userID != "" {
				reqLogger = reqLogger.UID(userID)
			}
			if tenantID != "" {
				reqLogger = reqLogger.Tenant(tenantID)
			}

			// Wrap response writer to capture status
			wrapped := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
				size:           0,
			}

			// Log request start
			reqLogger.Info("HTTP request started")

			// Handle request
			r = r.WithContext(ctx)
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Log request completion
			reqLogger.
				F("status", wrapped.status).
				F("size", wrapped.size).
				F("duration_ms", duration.Milliseconds()).
				Info("HTTP request completed")

			// Log error if status >= 500
			if wrapped.status >= 500 {
				reqLogger.Error("HTTP request failed with server error")
			}
		})
	}
}

// GinMiddleware creates Gin middleware for logging
func GinMiddleware(logger Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Ensure request exists for test safety
		if c.Request == nil {
			req, _ := http.NewRequest("GET", "/", nil)
			c.Request = req
		}

		// Extract or generate request ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = GenerateRequestID()
			c.Header("X-Request-ID", requestID)
		}

		// Store in context
		c.Set("request_id", requestID)

		// Extract user info
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("user_id", userID)
		}

		// Extract tenant info
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}

		// Create logger with context
		reqLogger := logger.
			Ctx(c.Request.Context()).
			RID(requestID).
			IP(c.ClientIP()).
			F("method", c.Request.Method).
			F("path", c.Request.URL.Path).
			F("user_agent", c.Request.UserAgent())

		if userID != "" {
			reqLogger = reqLogger.UID(userID)
		}
		if tenantID != "" {
			reqLogger = reqLogger.Tenant(tenantID)
		}

		// Store logger in context
		c.Set("logger", reqLogger)

		// Capture request body if needed
		var requestBody []byte
		if c.Request.Body != nil && shouldLogBody(c.Request.Header.Get("Content-Type")) {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
		}

		// Log request start
		if len(requestBody) > 0 {
			reqLogger.F("request_body", truncateBody(requestBody)).
				Info("HTTP request started")
		} else {
			reqLogger.Info("HTTP request started")
		}

		// Capture response
		blw := &bodyLogWriter{ResponseWriter: c.Writer, body: bytes.NewBufferString("")}
		c.Writer = blw

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Get response status
		status := c.Writer.Status()

		// Build final log
		finalLogger := reqLogger.
			F("status", status).
			F("size", c.Writer.Size()).
			F("duration_ms", duration.Milliseconds())

		// Add response body if needed
		if blw.body.Len() > 0 && shouldLogBody(c.Writer.Header().Get("Content-Type")) {
			finalLogger = finalLogger.F("response_body", truncateBody(blw.body.Bytes()))
		}

		// Log errors from Gin
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				finalLogger = finalLogger.Err(err)
			}
		}

		// Log based on status
		switch {
		case status >= 500:
			finalLogger.Error("HTTP request failed with server error")
		case status >= 400:
			finalLogger.Warn("HTTP request failed with client error")
		default:
			finalLogger.Info("HTTP request completed")
		}

		// Update metrics if available
		if provider, exists := c.Get("otel_provider"); exists {
			if otelProvider, ok := provider.(*OTelProvider); ok {
				otelProvider.DecrementActiveRequests(c.Request.Context())
			}
		}
	}
}

// OTelHTTPMiddleware combines OpenTelemetry and logging middleware
func OTelHTTPMiddleware(logger Logger, provider *OTelProvider, serviceName string) func(http.Handler) http.Handler {
	otelMiddleware := otelhttp.NewMiddleware(serviceName,
		otelhttp.WithTracerProvider(provider.tracerProvider),
		otelhttp.WithMeterProvider(provider.meterProvider),
	)

	return func(next http.Handler) http.Handler {
		return otelMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context
			ctx := provider.ExtractContext(r)
			span := trace.SpanFromContext(ctx)

			// Increment active requests
			provider.IncrementActiveRequests(ctx)
			defer provider.DecrementActiveRequests(ctx)

			// Create logger with trace context
			reqLogger := logger.Ctx(ctx)

			// Add span attributes
			if span.IsRecording() {
				span.SetAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.remote_addr", r.RemoteAddr),
				)
			}

			// Store logger in context
			ctx = context.WithValue(ctx, "logger", reqLogger)
			r = r.WithContext(ctx)

			// Call HTTP middleware
			HTTPMiddleware(reqLogger)(next).ServeHTTP(w, r)
		}))
	}
}

// OTelGinMiddleware combines OpenTelemetry and logging for Gin
func OTelGinMiddleware(logger Logger, provider *OTelProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start span
		ctx, span := provider.tracer.Start(c.Request.Context(),
			fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()),
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.user_agent", c.Request.UserAgent()),
				attribute.String("http.remote_addr", c.ClientIP()),
			),
		)
		defer span.End()

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		// Increment active requests
		provider.IncrementActiveRequests(ctx)
		defer provider.DecrementActiveRequests(ctx)

		// Create logger with trace context
		reqLogger := logger.Ctx(ctx)
		c.Set("logger", reqLogger)
		c.Set("otel_provider", provider)

		// Use Gin logging middleware
		GinMiddleware(reqLogger)(c)

		// Record span status based on response
		if c.Writer.Status() >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", c.Writer.Status()))
		}

		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int64("http.response_size", int64(c.Writer.Size())),
		)
	}
}

// Helper types

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.size += size
	return size, err
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Helper functions

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP
		if comma := bytes.IndexByte([]byte(xff), ','); comma != -1 {
			xff = xff[:comma]
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Use RemoteAddr
	return r.RemoteAddr
}

func shouldLogBody(contentType string) bool {
	// Log JSON and form data bodies
	return contentType == "application/json" ||
		contentType == "application/x-www-form-urlencoded" ||
		contentType == "multipart/form-data"
}

func truncateBody(body []byte) string {
	const maxLen = 1000
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "...(truncated)"
}

// GetLoggerFromContext retrieves logger from context
func GetLoggerFromContext(ctx context.Context) (Logger, bool) {
	logger, ok := ctx.Value("logger").(Logger)
	return logger, ok
}

// GetLoggerFromGinContext retrieves logger from Gin context
func GetLoggerFromGinContext(c *gin.Context) (Logger, bool) {
	logger, exists := c.Get("logger")
	if !exists {
		return nil, false
	}

	l, ok := logger.(Logger)
	return l, ok
}

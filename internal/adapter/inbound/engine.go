package inbound

import (
	pin "obs-brutal/internal/core/port/inbound"

	"github.com/gin-gonic/gin"
)

func NewGinEngine() *gin.Engine { r := gin.New(); r.Use(gin.Recovery()); return r }

// GinMiddleware provides a simple logging middleware using the provided logger
func GinMiddleware(logger pin.Logger) gin.HandlerFunc {
	return NewMiddlewareAdapter(logger).GinLoggingMiddleware()
}

// GetLoggerFromGinContext retrieves the logger from gin.Context if set by middleware
func GetLoggerFromGinContext(c *gin.Context) (pin.Logger, bool) {
	v, ok := c.Get("logger")
	if !ok {
		return nil, false
	}
	l, ok := v.(pin.Logger)
	return l, ok
}

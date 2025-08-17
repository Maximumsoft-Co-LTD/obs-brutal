package inbound

import (
	"fmt"
	"net/http"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/inbound"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTPAdapter provides HTTP endpoints for logging operations
type HTTPAdapter struct {
	loggingService       inbound.Service
	featureService       inbound.Features
	errorCategoryService inbound.ErrCategories
}

// NewHTTPAdapter creates a new HTTP adapter
func NewHTTPAdapter(
	loggingService inbound.Service,
	featureService inbound.Features,
	errorCategoryService inbound.ErrCategories,
) *HTTPAdapter {
	return &HTTPAdapter{
		loggingService:       loggingService,
		featureService:       featureService,
		errorCategoryService: errorCategoryService,
	}
}

// RegisterRoutes registers HTTP routes
func (a *HTTPAdapter) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/logging")
	{
		// Logging endpoints
		api.POST("/log", a.handleLog)
		api.POST("/error", a.handleLogErr)
		api.POST("/structured-error", a.handleLogStruct)

		// Configuration endpoints
		api.GET("/features", a.listFeatures)
		api.POST("/features", a.registerFeature)
		api.GET("/features/:name", a.getFeature)

		// Error categories
		api.GET("/error-categories", a.listErrorCategories)
		api.POST("/error-categories", a.registerErrorCategory)
		api.GET("/error-categories/:category", a.getErrorCategory)

		// Health check
		api.GET("/health", a.healthCheck)
	}
}

// handleLog handles log requests
func (a *HTTPAdapter) handleLog(c *gin.Context) {
	var req LogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse level
	level := parseHTTPLevel(req.Level)

	// Log with context
	ctx := c.Request.Context()
	if err := a.loggingService.Log(ctx, level, req.Message, req.Fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "logged"})
}

// handleLogErr handles error log requests
func (a *HTTPAdapter) handleLogErr(c *gin.Context) {
	var req ErrorLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create error
	err := fmt.Errorf("%s", req.Error)

	// Log error
	ctx := c.Request.Context()
	if err := a.loggingService.LogErr(ctx, err, req.Category, req.Details); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "error logged"})
}

// handleLogStruct handles structured error log requests
func (a *HTTPAdapter) handleLogStruct(c *gin.Context) {
	var req StructuredErrorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create structured error
	structErr := domain.StructuredError{
		Code:     req.Code,
		Message:  req.Message,
		Category: req.Category,
		Details:  req.Details,
	}

	if req.Cause != "" {
		structErr.Cause = fmt.Errorf("%s", req.Cause)
	}

	// Log structured error
	ctx := c.Request.Context()
	if err := a.loggingService.LogStruct(ctx, structErr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "structured error logged"})
}

// listFeatures lists available features
func (a *HTTPAdapter) listFeatures(c *gin.Context) {
	features := a.featureService.List()
	c.JSON(http.StatusOK, gin.H{"features": features})
}

// registerFeature registers a new feature
func (a *HTTPAdapter) registerFeature(c *gin.Context) {
	// Feature registration would typically be done programmatically
	// This endpoint is for demonstration
	c.JSON(http.StatusNotImplemented, gin.H{"error": "feature registration must be done programmatically"})
}

// getFeature gets a specific feature
func (a *HTTPAdapter) getFeature(c *gin.Context) {
	name := c.Param("name")

	feature, err := a.featureService.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name": feature.Name(),
	})
}

// listErrorCategories lists error categories
func (a *HTTPAdapter) listErrorCategories(c *gin.Context) {
	categories := a.errorCategoryService.List()
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// registerErrorCategory registers an error category
func (a *HTTPAdapter) registerErrorCategory(c *gin.Context) {
	// Category registration would typically be done programmatically
	// This endpoint is for demonstration
	c.JSON(http.StatusNotImplemented, gin.H{"error": "category registration must be done programmatically"})
}

// getErrorCategory gets an error category handler
func (a *HTTPAdapter) getErrorCategory(c *gin.Context) {
	category := c.Param("category")

	handler, err := a.errorCategoryService.Get(category)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category":     handler.Category(),
		"should_alert": handler.ShouldAlert(),
		"severity":     handler.Severity().String(),
	})
}

// healthCheck performs health check
func (a *HTTPAdapter) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// Request DTOs

// LogRequest represents a log request
type LogRequest struct {
	Level   string                 `json:"level" binding:"required"`
	Message string                 `json:"message" binding:"required"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// ErrorLogRequest represents an error log request
type ErrorLogRequest struct {
	Error    string                 `json:"error" binding:"required"`
	Category string                 `json:"category" binding:"required"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// StructuredErrorRequest represents a structured error request
type StructuredErrorRequest struct {
	Code     string                 `json:"code" binding:"required"`
	Message  string                 `json:"message" binding:"required"`
	Category string                 `json:"category" binding:"required"`
	Details  map[string]interface{} `json:"details,omitempty"`
	Cause    string                 `json:"cause,omitempty"`
}

// Helper functions

func parseHTTPLevel(level string) domain.Level {
	switch level {
	case "debug", "DEBUG":
		return domain.DebugLevel
	case "info", "INFO":
		return domain.InfoLevel
	case "warn", "WARN", "warning", "WARNING":
		return domain.WarnLevel
	case "error", "ERROR":
		return domain.ErrorLevel
	case "fatal", "FATAL":
		return domain.FatalLevel
	default:
		return domain.InfoLevel
	}
}

package main

import (
	"fmt"
	"log"
	"obs-brutal/pkg/obsvbrutal"
	"time"

	"github.com/gin-gonic/gin"
)

// ตัวอย่างการใช้งาน Simple Interface (GetLogFrmGin)
func main() {
	// สร้าง logger
	logger, err := obsvbrutal.NewLogger(
		obsvbrutal.WithLevel(obsvbrutal.InfoLevel),
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// สร้าง Gin router
	router := gin.New()
	router.Use(obsvbrutal.GinMiddleware(logger))

	// Example endpoints
	router.GET("/demo/simple", demoSimpleInterface)
	router.GET("/demo/error", demoErrorHandling)
	router.GET("/demo/trace", demoTracing)
	router.GET("/demo/response", demoResponseBuilder)
	router.GET("/demo/dev", demoDevelopmentFeatures)

	log.Println("Starting server on :8080")
	router.Run(":8080")
}

// 1. Field management: F, Fs
func demoSimpleInterface(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "SimpleDemo")
	defer log.Close()

	// Single field
	log.F("action", "demo_start").Prt("Starting simple interface demo")

	// Multiple fields
	log.F("user_id", c.Query("user_id")).
		F("session_id", c.GetHeader("X-Session-ID")).
		F("ip", c.ClientIP()).
		Prt("Request context captured")

	// Chain fields
	log.
		F("step", 1).
		F("description", "validation").
		Prt("Processing step")

	c.JSON(200, gin.H{"status": "ok"})
}

// 2. Error handling: Err, EC (Error with Category)
func demoErrorHandling(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "ErrorDemo")
	defer log.Close()

	// Simulate different error scenarios
	errorType := c.Query("type")

	switch errorType {
	case "validation":
		err := fmt.Errorf("invalid input: field 'email' is required")
		log.Err(err)
		log.Err(err)
		log.Prt("Validation error")
		c.JSON(400, gin.H{"error": err.Error()})

	case "business":
		err := fmt.Errorf("insufficient balance")
		log.Err(err)
		log.Err(err)
		log.Prt("Business error")
		c.JSON(422, gin.H{"error": err.Error()})

	case "system":
		err := fmt.Errorf("database connection failed")
		log.Err(err)
		log.Err(err)
		log.Prt("System error")
		c.JSON(500, gin.H{"error": err.Error()})

	default:
		// Structured error
		structErr := obsvbrutal.StructuredError{
			Code:     "APP_001",
			Message:  "Unknown error type",
			Category: "unknown",
			Details: map[string]interface{}{
				"requested_type": errorType,
			},
		}
		log.Err(fmt.Errorf("%s: %s", structErr.Code, structErr.Message))
		var opts = obsvbrutal.OptsResponse()
		log.R(400, opts.Msg("Unknown Error"), opts.Detail(structErr.Error()))
	}
}

// 3. Tracing: Parent, Close
func demoTracing(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "TraceDemo")
	defer log.Close()

	log.Prt("Starting trace demo")

	// Create parent trace
	dbTrace := log.FlatPr("database.query")
	dbTrace.Add(
		dbTrace.Str("query", "SELECT * FROM users"),
		dbTrace.Num("limit", 10),
	)

	// Simulate database operation
	time.Sleep(50 * time.Millisecond)
	dbTrace.End()

	// Create another trace for external API
	apiTrace := log.FlatPr("external.api.call")
	apiTrace.Add(
		apiTrace.Str("endpoint", "https://api.example.com/users"),
		apiTrace.Code(200),
	)

	// Simulate API call
	time.Sleep(100 * time.Millisecond)

	// Handle error in trace
	if err := simulateAPIError(); err != nil {
		apiTrace.Err(err)
	}
	apiTrace.End()
	var opts = obsvbrutal.OptsResponse()
	log.R(200, opts.Msg("Trace demo completed"), opts.Response(gin.H{
		"traces": []string{"database.query", "external.api.call"},
	}))
}

// 4. Response building: R
func demoResponseBuilder(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "ResponseDemo")
	defer log.Close()

	action := c.Query("action")

	switch action {
	case "success":
		// Success response with data
		var opts = obsvbrutal.OptsResponse()
		log.R(200, opts.Msg("Operation successful"), opts.Response(gin.H{
			"user_id": "123",
			"name":    "John Doe",
			"email":   "john@example.com",
		}))

	case "created":
		// Created response
		var opts = obsvbrutal.OptsResponse()
		log.R(201, opts.Msg("Resource created"), opts.Response(gin.H{
			"id":         "new-123",
			"created_at": time.Now(),
		}))

	case "error":
		// Error response
		err := fmt.Errorf("something went wrong")
		var opts = obsvbrutal.OptsResponse()
		log.R(500, opts.Msg("Internal server error"), opts.Detail(err.Error()))

	default:
		// Not found
		log.R(404, obsvbrutal.OptsResponse().
			Msg("Action not found"))
	}
}

// 5. Development features
func demoDevelopmentFeatures(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Development features demo"})
}

// Helper function
func simulateAPIError() error {
	// 30% chance of error
	if time.Now().UnixNano()%10 < 3 {
		return fmt.Errorf("API timeout")
	}
	return nil
}

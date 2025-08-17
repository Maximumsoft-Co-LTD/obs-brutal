package main

import (
	"context"
	"fmt"
	"log"
	"obs-brutal/pkg/obsvbrutal"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

// ตัวอย่างการใช้งาน Trace Interface แบบครบทุก method
func main() {
	// Initialize logger
	logger, err := obsvbrutal.NewLogger(
		obsvbrutal.WithLevel(obsvbrutal.InfoLevel),
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// Initialize OpenTelemetry
	otelProvider, err := obsvbrutal.NewOTelProvider(
		"trace-example",
		"localhost:4317",
		true,
	)
	if err != nil {
		log.Println("Warning: OTLP not available, continuing without traces")
	} else {
		defer otelProvider.Shutdown(context.Background())
	}

	// Setup Gin
	router := gin.New()
	router.Use(obsvbrutal.GinMiddleware(logger))

	// Trace examples
	router.GET("/trace/nested", demoNestedTraces)
	router.GET("/trace/attributes", demoTraceAttributes)
	router.GET("/trace/errors", demoTraceErrors)
	router.GET("/trace/complete", demoCompleteFlow)

	log.Println("Starting server on :8080")
	router.Run(":8080")
}

// 1. Nested traces: Parent creating child traces
func demoNestedTraces(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "NestedTraceDemo")
	defer log.Close()

	// Root trace
	rootTrace := log.FlatPr("order.process")
	rootTrace.Add(
		rootTrace.Str("order_id", "ORD-123"),
		rootTrace.Num("total_amount", 1500.50),
	)

	// Child trace 1: Validate order
	validateTrace := rootTrace.FlatPr("order.validate")
	validateTrace.Add(
		validateTrace.Bool("has_stock", true),
		validateTrace.Bool("payment_valid", true),
	)
	time.Sleep(20 * time.Millisecond)
	validateTrace.End()

	// Child trace 2: Process payment
	paymentTrace := rootTrace.FlatPr("payment.process")
	paymentTrace.Add(
		paymentTrace.Str("payment_method", "credit_card"),
		paymentTrace.Str("transaction_id", "TXN-456"),
	)
	time.Sleep(50 * time.Millisecond)
	paymentTrace.End()

	// Child trace 3: Update inventory
	inventoryTrace := rootTrace.FlatPr("inventory.update")
	inventoryTrace.Add(
		inventoryTrace.Num("items_updated", 3),
		inventoryTrace.Str("products", "PROD-1,PROD-2,PROD-3"),
	)
	time.Sleep(30 * time.Millisecond)
	inventoryTrace.End()

	rootTrace.End()

	c.JSON(200, gin.H{
		"status": "Order processed with nested traces",
		"traces": []string{
			"order.process",
			"order.validate",
			"payment.process",
			"inventory.update",
		},
	})
}

// 2. Trace attributes: All attribute methods
func demoTraceAttributes(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "AttributesDemo")
	defer log.Close()

	trace := log.FlatPr("attributes.demo")

	// String attributes
	trace.Add(trace.Str("user_name", "John Doe"))
	trace.Add(trace.Str("email", "john@example.com"))

	// Boolean attributes
	trace.Add(trace.Bool("is_premium", true))
	trace.Add(trace.Bool("email_verified", false))

	// Numeric attributes
	trace.Add(trace.Num("account_balance", 2500.75))
	trace.Add(trace.Num("credit_score", 750))

	// Complex body (any type)
	trace.Add(trace.Str("user_profile", "preferences:language:th,timezone:Asia/Bangkok,theme:dark,metadata:created_at:2021-01-01,last_login:2021-01-01"))

	// Message and detail
	trace.Add(trace.Msg("User profile loaded successfully"))
	trace.Add(trace.Detail("Loaded from cache with TTL 3600s"))

	// Status code
	trace.Add(trace.Code(200))

	// Multiple attributes at once
	trace.Add(
		attribute.String("country", "TH"),
		attribute.Int("age", 25),
		attribute.Float64("lat", 13.7563),
		attribute.Float64("lng", 100.5018),
		attribute.StringSlice("interests", []string{"tech", "travel", "food"}),
	)

	trace.End()

	c.JSON(200, gin.H{
		"status": "Demonstrated all attribute types",
	})
}

// 3. Error handling in traces: Err, Errf
func demoTraceErrors(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "ErrorTraceDemo")
	defer log.Close()

	scenario := c.Query("scenario")

	trace := log.FlatPr("error.handling")
	trace.Add(trace.Str("scenario", scenario))

	switch scenario {
	case "simple":
		// Simple error
		err := fmt.Errorf("connection timeout")
		if traceErr := trace.Err(err); traceErr != nil {
			log.Err(traceErr)
		}

	case "formatted":
		// Formatted error
		userID := "user-123"
		action := "withdraw"
		amount := 5000.0

		if traceErr := trace.Errf("User %s cannot %s amount %.2f: insufficient balance",
			userID, action, amount); traceErr != nil {
			log.Err(traceErr)
		}

	case "chain":
		// Error chain
		baseErr := fmt.Errorf("database connection failed")
		wrapErr := fmt.Errorf("failed to fetch user: %w", baseErr)
		finalErr := fmt.Errorf("authentication failed: %w", wrapErr)

		if traceErr := trace.Err(finalErr); traceErr != nil {
			log.Err(traceErr)
		}

	default:
		trace.Add(trace.Msg("No error scenario selected"))
	}

	trace.End()

	c.JSON(200, gin.H{
		"status":   "Error handling demonstrated",
		"scenario": scenario,
	})
}

// 4. Complete flow example
func demoCompleteFlow(c *gin.Context) {
	log := obsvbrutal.GetLogFrmGin(c, "CompleteFlowDemo")
	defer log.Close()

	// Main operation trace
	mainTrace := log.FlatPr("user.registration")
	mainTrace.Add(
		mainTrace.Str("flow", "complete_registration"),
		mainTrace.Str("request", "username:newuser,email:newuser@example.com,source:web"),
	)

	// Step 1: Validation
	validationTrace := mainTrace.FlatPr("validation")
	validationTrace.Add(validationTrace.Msg("Validating user input"))

	if err := validateUserInput("newuser", "newuser@example.com"); err != nil {
		validationTrace.Err(err)
		validationTrace.End()
		mainTrace.End()
		var opts = obsvbrutal.OptsResponse()
		log.R(400, opts.Msg("Validation failed"), opts.Detail(err.Error())).Err(err)
		return
	}

	validationTrace.Add(validationTrace.Bool("valid", true))
	validationTrace.End()

	// Step 2: Check duplicates
	dupCheckTrace := mainTrace.FlatPr("duplicate.check")
	dupCheckTrace.Add(
		dupCheckTrace.Str("check_type", "email_and_username"),
	)
	time.Sleep(30 * time.Millisecond) // Simulate DB check
	dupCheckTrace.Add(
		dupCheckTrace.Bool("username_exists", false),
		dupCheckTrace.Bool("email_exists", false),
	)
	dupCheckTrace.End()

	// Step 3: Create user
	createTrace := mainTrace.FlatPr("user.create")
	userID := fmt.Sprintf("user-%d", time.Now().Unix())
	createTrace.Add(
		createTrace.Str("user_id", userID),
		createTrace.Num("shard_id", float64(time.Now().Unix()%10)),
	)
	time.Sleep(50 * time.Millisecond) // Simulate DB insert
	createTrace.End()

	// Step 4: Send welcome email
	emailTrace := mainTrace.FlatPr("email.send")
	emailTrace.Add(
		emailTrace.Str("template", "welcome"),
		emailTrace.Str("to", "newuser@example.com"),
	)

	if err := sendWelcomeEmail("newuser@example.com"); err != nil {
		// Non-critical error, just log it
		emailTrace.Err(err)
		log.Err(err)
	} else {
		emailTrace.Add(emailTrace.Bool("sent", true))
	}
	emailTrace.End()

	// Complete main trace
	mainTrace.Add(
		mainTrace.Bool("success", true),
		mainTrace.Str("user_id", userID),
		mainTrace.Detail("User registration completed successfully"),
		mainTrace.Code(201),
	)
	mainTrace.End()

	var opts = obsvbrutal.OptsResponse()
	log.R(201, opts.Msg("User registered successfully"), opts.Response(gin.H{
		"user_id": userID,
		"email":   "newuser@example.com",
	}))
}

// Helper functions
func validateUserInput(username, email string) error {
	if len(username) < 3 {
		return fmt.Errorf("username too short")
	}
	if len(email) < 5 || !contains(email, "@") {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func sendWelcomeEmail(email string) error {
	// Simulate 20% failure rate
	if time.Now().UnixNano()%10 < 2 {
		return fmt.Errorf("email service unavailable")
	}
	return nil
}

func contains(s, substr string) bool {
	for i := 0; i < len(s); i++ {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

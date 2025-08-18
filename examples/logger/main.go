package main

import (
	"context"
	"log"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"
)

// ตัวอย่างการใช้งาน Logger Interface แบบครบทุก method
func main() {
	// สร้าง logger
	logger, err := logbrutal.NewLogger(
		logbrutal.WithLevel(logbrutal.InfoLevel),
		logbrutal.WithSinks(
			logbrutal.NewStdoutSink(),
			logbrutal.NewFileSink("app.log", 100, 30, 10, true),
		),
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// 1. การใช้งาน Context methods
	demoContextMethods(logger)

	// 2. การใช้งาน Correlation IDs
	demoCorrelationIDs(logger)

	// 3. การใช้งาน Logging methods
	demoLoggingMethods(logger)

	// 4. การใช้งาน Configuration
	demoConfiguration(logger)

	// 5. การใช้งาน Metrics
	demoMetrics(logger)
}

// 1. Context methods: Ctx, F, Fs, Err
func demoContextMethods(logger logbrutal.Logger) {
	log.Println("\n=== Context Methods Demo ===")

	// Context with trace
	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	ctxLogger := logger.Ctx(ctx)
	ctxLogger.Info("Log with context")

	// Single field
	logger.F("user_id", "12345").Info("Log with single field")

	// Multiple fields
	logger.Fs(map[string]interface{}{
		"order_id": "ORD-789",
		"amount":   99.99,
		"currency": "THB",
	}).Info("Log with multiple fields")

	// With error
	err := processOrder()
	if err != nil {
		logger.Err(err).Error("Failed to process order")
	}
}

// 2. Correlation IDs: TID, SID, UID, RID, IP, Sess, Tenant, Mod
func demoCorrelationIDs(logger logbrutal.Logger) {
	log.Println("\n=== Correlation IDs Demo ===")

	// Complete request tracking
	requestLogger := logger.
		TID("550e8400-e29b-41d4-a716-446655440000"). // Trace ID
		SID("123e4567-e89b-12d3-a456-426614174000"). // Span ID
		UID("user-456").                             // User ID
		RID("req-789").                              // Request ID
		IP("192.168.1.100").                         // Client IP
		Sess("sess-abc123").                         // Session ID
		Tenant("tenant-xyz").                        // Tenant ID
		Mod("order-service")                         // Module name

	requestLogger.Info("Processing order request")
}

// 3. Logging methods: Debug, Info, Warn, Error, Fatal
func demoLoggingMethods(logger logbrutal.Logger) {
	log.Println("\n=== Logging Methods Demo ===")

	// Different log levels
	logger.Debug("Debug message - detailed information")
	logger.Info("Info message - general information")
	logger.Warn("Warning message - something might be wrong")
	logger.Error("Error message - something went wrong")
	// logger.Fatal("Fatal message - application will exit") // ระวัง: จะ exit app

	// With additional fields using domain.Field
	logger.Info("Order processed",
		logbrutal.Field{Key: "order_id", Value: "ORD-123"},
		logbrutal.Field{Key: "status", Value: "completed"},
	)
}

// 4. Configuration: Level, GetLevel
func demoConfiguration(logger logbrutal.Logger) {
	log.Println("\n=== Configuration Demo ===")

	// Get current level
	currentLevel := logger.GetLevel()
	log.Printf("Current log level: %v", currentLevel)

	// Change level at runtime
	logger.Level(logbrutal.DebugLevel)
	logger.Debug("Now debug logs are visible")

	// Change back
	logger.Level(logbrutal.InfoLevel)
	logger.Debug("This debug log won't show")
	logger.Info("But info logs still show")
}

// 5. Metrics: Logged, Filtered
func demoMetrics(logger logbrutal.Logger) {
	log.Println("\n=== Metrics Demo ===")

	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Info("Test message", logbrutal.Field{Key: "index", Value: i})
	}

	// Get metrics
	loggedCount := logger.Logged()
	filteredCount := logger.Filtered()

	log.Printf("Total logged: %d", loggedCount)
	log.Printf("Total filtered: %d", filteredCount)
}

// Helper function
func processOrder() error {
	// Simulate error
	return &OrderError{
		Code:    "ORD_001",
		Message: "Insufficient inventory",
	}
}

// Custom error type
type OrderError struct {
	Code    string
	Message string
}

func (e *OrderError) Error() string {
	return e.Message
}

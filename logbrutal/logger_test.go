package logbrutal_test

import (
	"context"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"
)

// TestLoggerInterface ทดสอบ Logger interface methods
func TestLoggerInterface(t *testing.T) {
	// สร้าง test logger
	logger := logbrutal.NewMockLogger()

	t.Run("Context Methods", func(t *testing.T) {
		// Test Ctx
		ctx := context.WithValue(context.Background(), "test", "value")
		ctxLogger := logger.Ctx(ctx)
		if ctxLogger == nil {
			t.Error("Ctx() should return a logger")
		}

		// Test F (single field)
		fieldLogger := logger.F("key", "value")
		if fieldLogger == nil {
			t.Error("F() should return a logger")
		}

		// Test Fs (multiple fields)
		fieldsLogger := logger.Fs(map[string]interface{}{
			"field1": "value1",
			"field2": 123,
			"field3": true,
		})
		if fieldsLogger == nil {
			t.Error("Fs() should return a logger")
		}

		// Test Err
		err := &testError{msg: "test error"}
		errLogger := logger.Err(err)
		if errLogger == nil {
			t.Error("Err() should return a logger")
		}
	})

	t.Run("Correlation IDs", func(t *testing.T) {
		// Test all correlation ID methods
		correlationLogger := logger.
			TID("trace-123").
			SID("span-456").
			UID("user-789").
			RID("req-abc").
			IP("192.168.1.1").
			Sess("session-xyz").
			Tenant("tenant-1").
			Mod("test-module")

		if correlationLogger == nil {
			t.Error("Correlation ID methods should return a logger")
		}
	})

	t.Run("Logging Methods", func(t *testing.T) {
		// Test different log levels
		logger.Debug("Debug message")
		logger.Info("Info message")
		logger.Warn("Warning message")
		logger.Error("Error message")
		// Note: Not testing Fatal as it would exit

		// Test with fields
		fields := []domain.Field{
			{Key: "field1", Value: "value1"},
			{Key: "field2", Value: 123},
		}
		logger.Info("Message with fields", fields...)

		// Verify logged count
		if logger.Logged() == 0 {
			t.Error("Should have logged messages")
		}
	})

	t.Run("Configuration", func(t *testing.T) {
		// Test Level setting
		logger.Level(domain.DebugLevel)
		if logger.GetLevel() != domain.DebugLevel {
			t.Errorf("Expected level %v, got %v", domain.DebugLevel, logger.GetLevel())
		}

		// Change to higher level
		logger.Level(domain.ErrorLevel)
		if logger.GetLevel() != domain.ErrorLevel {
			t.Errorf("Expected level %v, got %v", domain.ErrorLevel, logger.GetLevel())
		}
	})

	t.Run("Metrics", func(t *testing.T) {
		// Reset counts
		logger = logbrutal.NewMockLogger()

		// Log some messages
		logger.Info("Test 1")
		logger.Debug("Test 2")
		logger.Error("Test 3")

		// Check logged count
		logged := logger.Logged()
		if logged != 3 {
			t.Errorf("Expected 3 logged messages, got %d", logged)
		}

		// Set level to ERROR and log more
		logger.Level(domain.ErrorLevel)
		logger.Info("Should be filtered")
		logger.Debug("Should be filtered")
		logger.Error("Should pass")

		// Check filtered count
		filtered := logger.Filtered()
		if filtered != 2 {
			t.Errorf("Expected 2 filtered messages, got %d", filtered)
		}
	})

	t.Run("Chaining", func(t *testing.T) {
		// Test method chaining
		chainedLogger := logger.
			F("app", "test").
			F("version", "1.0.0").
			UID("user-123").
			TID("trace-456").
			Mod("chain-test")

		chainedLogger.Info("Chained logger test")

		// Verify it logged
		if logger.Logged() == 0 {
			t.Error("Chained logger should have logged")
		}
	})
}

// TestLoggerWithRealImplementation ทดสอบกับ implementation จริง
func TestLoggerWithRealImplementation(t *testing.T) {
	// สร้าง logger จริง
	logger, err := logbrutal.NewLogger(
		logbrutal.WithLevel(logbrutal.InfoLevel),
		logbrutal.WithSinks(logbrutal.NewStdoutSink()),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	t.Run("Real Logging", func(t *testing.T) {
		// Test real logging
		logger.Info("Real implementation test")

		// With fields
		logger.
			F("test", true).
			F("timestamp", time.Now()).
			Info("Test with fields")

		// With error
		err := &testError{msg: "real error"}
		logger.Err(err).Error("Error occurred")
	})

	t.Run("Performance", func(t *testing.T) {
		start := time.Now()

		// Log 1000 messages
		for i := 0; i < 1000; i++ {
			logger.
				F("index", i).
				F("batch", i/100).
				Info("Performance test message")
		}

		duration := time.Since(start)
		t.Logf("Logged 1000 messages in %v", duration)

		// Should be fast (< 100ms for 1000 logs)
		if duration > 100*time.Millisecond {
			t.Logf("Warning: Logging is slower than expected: %v", duration)
		}
	})
}

// TestSafeLogger ทดสอบ SafeLogger ที่ป้องกัน panic
func TestSafeLogger(t *testing.T) {
	// สร้าง logger ที่อาจ panic
	var nilLogger logbrutal.Logger

	// Wrap with safe logger
	safeLogger := logbrutal.NewSafeLogger(nilLogger)

	t.Run("No Panic", func(t *testing.T) {
		// These should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SafeLogger should not panic: %v", r)
			}
		}()

		// Try all methods
		safeLogger.Info("Test")
		safeLogger.F("key", "value")
		safeLogger.Err(nil)
		safeLogger.TID("123")
		safeLogger.Level(logbrutal.DebugLevel)
		safeLogger.GetLevel()
		safeLogger.Logged()
	})
}

// Helper types
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

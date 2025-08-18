package logbrutal_test

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"

	"github.com/gin-gonic/gin"
)

// TestGinLoggerInterface ทดสอบ GinLogger interface (GetLogFrmGin)
func TestGinLoggerInterface(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// สร้าง logger
	logger, err := logbrutal.NewLogger(
		logbrutal.WithLevel(logbrutal.InfoLevel),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	t.Run("GetLogFrmGin", func(t *testing.T) {
		// สร้าง test context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		// ใช้ middleware
		logbrutal.GinMiddleware(logger)(c)

		// Get Simple logger
		log := logbrutal.GetLogFrmGin(c, "TestOperation")
		defer log.Close()

		if log == nil {
			t.Fatal("GetLogFrmGin should return a logger")
		}
	})

	t.Run("Field Management", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "FieldTest")
		defer log.Close()

		// Test F (single field)
		log.F("key1", "value1").Prt("Single field test")

		// Test multiple F calls
		log.
			F("name", "John").
			F("age", 30).
			F("active", true).
			Prt("Multiple fields test")

		// Test chaining
		log.
			F("step", 1).
			F("action", "validation").
			F("result", "success").
			Prt("Chained fields test")
	})

	t.Run("Logging Methods", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "LogTest")
		defer log.Close()

		// Test Prt
		log.Prt("Simple message")

		// Test Prtf
		log.Prtf("User %s performed action %s at %v", "john", "login", time.Now())
	})

	t.Run("Error Handling", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "ErrorTest")
		defer log.Close()

		// Test Err
		err := fmt.Errorf("test error")
		resultErr := log.Err(err)
		if resultErr == nil {
			t.Error("Err() should return error")
		}

		// Test Errf
		formattedErr := log.Errf("Error occurred: %s", "test error")
		if formattedErr == nil {
			t.Error("Errf() should return error")
		}
	})

	t.Run("Response Building", func(t *testing.T) {
		// สร้าง router สำหรับ test
		router := gin.New()
		router.Use(logbrutal.GinMiddleware(logger))

		router.GET("/test/response/:status", func(c *gin.Context) {
			log := logbrutal.GetLogFrmGin(c, "ResponseTest")
			defer log.Close()

			status := c.Param("status")

			switch status {
			case "success":
				// Simple response
				resp := log.R(200)
				if resp == nil {
					c.JSON(500, gin.H{"error": "Failed to create response"})
					return
				}
				c.JSON(200, gin.H{"status": "success"})

			case "error":
				// Error response
				resp := log.R(500)
				if resp == nil {
					c.JSON(500, gin.H{"error": "Failed to create response"})
					return
				}
				c.JSON(500, gin.H{"status": "error"})

			case "notfound":
				// Not found
				resp := log.R(404)
				if resp == nil {
					c.JSON(500, gin.H{"error": "Failed to create response"})
					return
				}
				c.JSON(404, gin.H{"status": "not found"})
			}
		})

		// Test success response
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test/response/success", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Test error response
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/test/response/error", nil)
		router.ServeHTTP(w, req)

		if w.Code != 500 {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})

	t.Run("Tracing", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "TraceTest")
		defer log.Close()

		// Create parent trace
		trace := log.FlatPr("operation.main")
		if trace == nil {
			t.Fatal("Parent() should return a tracer")
		}

		// Add attributes
		trace.Add(
			trace.Str("operation", "test"),
			trace.Num("duration", 100),
			trace.Bool("success", true),
		)

		// Create child trace
		childTrace := trace.ChildPr("operation.child")

		// Add single body attribute
		bodyAttrs := trace.Body("data", map[string]interface{}{
			"items": []string{"item1", "item2"},
			"count": 2,
		})
		// Body returns []attribute.KeyValue, so add them one by one
		for _, attr := range bodyAttrs {
			childTrace.Add(attr)
		}

		childTrace.End()

		// End parent trace
		trace.End()
	})

	t.Run("TraceID and SpanID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "IDTest")
		defer log.Close()

		// Get TraceID and SpanID
		traceID := log.GetTraceID()
		spanID := log.GetSpanID()

		// These might be empty if no OTLP provider is configured
		t.Logf("TraceID: %s", traceID)
		t.Logf("SpanID: %s", spanID)
	})
}

// TestGinLoggerEdgeCases ทดสอบ edge cases
func TestGinLoggerEdgeCases(t *testing.T) {
	logger, _ := logbrutal.NewLogger(logbrutal.WithLevel(logbrutal.InfoLevel))

	t.Run("Nil Values", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "NilTest")
		defer log.Close()

		// Test with nil error
		nilErr := log.Err(nil)
		if nilErr != nil {
			t.Error("Err(nil) should return nil")
		}

		// Test with nil field value
		log.F("nil_value", nil).Prt("Nil field test")
	})

	t.Run("Large Data", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "LargeDataTest")
		defer log.Close()

		// Large string
		largeString := make([]byte, 10000)
		for i := range largeString {
			largeString[i] = 'a'
		}

		log.F("large_data", string(largeString)).Prt("Large data test")

		// Many fields
		l := log
		for i := 0; i < 100; i++ {
			l = l.F(fmt.Sprintf("field_%d", i), i)
		}
		l.Prt("Many fields test")
	})

	t.Run("Concurrent Usage", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		logbrutal.GinMiddleware(logger)(c)

		log := logbrutal.GetLogFrmGin(c, "ConcurrentTest")
		defer log.Close()

		// Test concurrent logging
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				log.F("goroutine", id).Prt("Concurrent log")
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Benchmark GinLogger interface
func BenchmarkGinLoggerInterface(b *testing.B) {
	gin.SetMode(gin.TestMode)
	logger, _ := logbrutal.NewLogger(logbrutal.WithLevel(logbrutal.InfoLevel))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	logbrutal.GinMiddleware(logger)(c)

	b.Run("Simple Log", func(b *testing.B) {
		log := logbrutal.GetLogFrmGin(c, "Benchmark")
		defer log.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			log.Prt("Benchmark message")
		}
	})

	b.Run("With Fields", func(b *testing.B) {
		log := logbrutal.GetLogFrmGin(c, "Benchmark")
		defer log.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			log.F("index", i).F("batch", i/100).Prt("Benchmark with fields")
		}
	})

	b.Run("Error Handling", func(b *testing.B) {
		log := logbrutal.GetLogFrmGin(c, "Benchmark")
		defer log.Close()

		err := fmt.Errorf("benchmark error")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = log.Err(err)
		}
	})
}

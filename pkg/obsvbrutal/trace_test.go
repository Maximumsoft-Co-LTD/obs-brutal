package obsvbrutal_test

import (
	"fmt"
	"testing"
	"time"

	"obs-brutal/pkg/obsvbrutal"

	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

// TestTraceInterface ทดสอบ Trace interface
func TestTraceInterface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := obsvbrutal.NewLogger(obsvbrutal.WithLevel(obsvbrutal.InfoLevel))

	t.Run("Basic Trace", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		obsvbrutal.GinMiddleware(logger)(c)

		log := obsvbrutal.GetLogFrmGin(c, "TraceTest")
		defer log.Close()

		// Create trace
		trace := log.FlatPr("test.operation")
		if trace == nil {
			t.Fatal("Parent() should return a trace")
		}

		// End trace
		trace.End()
	})

	t.Run("Nested Traces", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		obsvbrutal.GinMiddleware(logger)(c)

		log := obsvbrutal.GetLogFrmGin(c, "NestedTraceTest")
		defer log.Close()

		// Root trace
		rootTrace := log.FlatPr("root.operation")

		// Child trace 1
		child1 := rootTrace.FlatPr("child.operation1")
		time.Sleep(10 * time.Millisecond)
		child1.End()

		// Child trace 2
		child2 := rootTrace.FlatPr("child.operation2")

		// Grandchild trace
		grandchild := child2.FlatPr("grandchild.operation")
		time.Sleep(5 * time.Millisecond)
		grandchild.End()

		child2.End()
		rootTrace.End()
	})

	t.Run("Trace Attributes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		obsvbrutal.GinMiddleware(logger)(c)

		log := obsvbrutal.GetLogFrmGin(c, "AttributeTest")
		defer log.Close()

		trace := log.FlatPr("attribute.test")

		// Test Str
		strAttr := trace.Str("name", "test-value")
		if strAttr.Key != "name" {
			t.Errorf("Expected key 'name', got '%s'", strAttr.Key)
		}

		// Test Bool
		boolAttr := trace.Bool("enabled", true)
		if boolAttr.Key != "enabled" {
			t.Errorf("Expected key 'enabled', got '%s'", boolAttr.Key)
		}

		// Test Num
		numAttr := trace.Num("count", 42.5)
		if numAttr.Key != "count" {
			t.Errorf("Expected key 'count', got '%s'", numAttr.Key)
		}

		// Test Body (complex object)
		bodyData := map[string]interface{}{
			"user": map[string]interface{}{
				"id":    "123",
				"name":  "John",
				"roles": []string{"admin", "user"},
			},
		}
		bodyAttrs := trace.Body("request", bodyData)
		// Body returns []attribute.KeyValue
		if len(bodyAttrs) == 0 {
			t.Error("Body() should return attributes")
		}

		// Test Detail
		detailAttr := trace.Detail("Processing user request")
		if detailAttr.Key != "detail" {
			t.Errorf("Expected key 'detail', got '%s'", detailAttr.Key)
		}

		// Test Msg
		msgAttr := trace.Msg("Operation completed")
		if msgAttr.Key != "message" {
			t.Errorf("Expected key 'message', got '%s'", msgAttr.Key)
		}

		// Test Code
		codeAttr := trace.Code(200)
		if codeAttr.Key != "code" {
			t.Errorf("Expected key 'code', got '%s'", codeAttr.Key)
		}

		// Add all attributes
		trace.Add(strAttr, boolAttr, numAttr)
		// Add body attributes separately
		trace.Add(bodyAttrs...)
		trace.Add(detailAttr, msgAttr, codeAttr)

		trace.End()
	})

	t.Run("Multiple Attributes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		obsvbrutal.GinMiddleware(logger)(c)

		log := obsvbrutal.GetLogFrmGin(c, "MultiAttrTest")
		defer log.Close()

		trace := log.FlatPr("multi.attribute")

		// Add multiple attributes at once
		trace.Add(
			trace.Str("service", "api"),
			trace.Str("version", "1.0.0"),
			trace.Num("latency_ms", 125.5),
			trace.Bool("cached", false),
			trace.Code(201),
			attribute.String("environment", "test"),
			attribute.Int("retry_count", 3),
			attribute.Float64("success_rate", 0.95),
			attribute.StringSlice("features", []string{"auth", "logging", "metrics"}),
		)

		trace.End()
	})

	t.Run("Error Handling", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		obsvbrutal.GinMiddleware(logger)(c)

		log := obsvbrutal.GetLogFrmGin(c, "ErrorTraceTest")
		defer log.Close()

		trace := log.FlatPr("error.test")

		// Test Err
		err := fmt.Errorf("test error")
		resultErr := trace.Err(err)
		if resultErr == nil || resultErr.Error() != "test error" {
			t.Error("Err() should return the same error")
		}

		// Test Errf
		formattedErr := trace.Errf("Failed to process %s with code %d", "request", 500)
		if formattedErr == nil {
			t.Error("Errf() should return an error")
		}
		expectedMsg := "Failed to process request with code 500"
		if formattedErr.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, formattedErr.Error())
		}

		trace.End()
	})
}

// TestTracePerformance ทดสอบ performance ของ trace
func TestTracePerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := obsvbrutal.NewLogger(obsvbrutal.WithLevel(obsvbrutal.InfoLevel))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	obsvbrutal.GinMiddleware(logger)(c)

	log := obsvbrutal.GetLogFrmGin(c, "PerfTest")
	defer log.Close()

	t.Run("Many Traces", func(t *testing.T) {
		start := time.Now()

		// Create 100 traces
		for i := 0; i < 100; i++ {
			trace := log.FlatPr(fmt.Sprintf("trace.%d", i))
			trace.Add(
				trace.Num("index", float64(i)),
				trace.Bool("even", i%2 == 0),
			)
			trace.End()
		}

		duration := time.Since(start)
		t.Logf("Created 100 traces in %v", duration)

		// Should be fast
		if duration > 50*time.Millisecond {
			t.Logf("Warning: Trace creation is slower than expected: %v", duration)
		}
	})

	t.Run("Deep Nesting", func(t *testing.T) {
		start := time.Now()

		// Create deeply nested traces (10 levels)
		var currentTrace obsvbrutal.Tracer
		currentTrace = log.FlatPr("level.0")

		for i := 1; i < 10; i++ {
			currentTrace = currentTrace.FlatPr(fmt.Sprintf("level.%d", i))
		}

		// End all traces (in reverse order)
		for i := 0; i < 10; i++ {
			currentTrace.End()
		}

		duration := time.Since(start)
		t.Logf("Created 10-level nested traces in %v", duration)
	})
}

// TestResponseInterface ทดสอบ Response interface
func TestResponseInterface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := obsvbrutal.NewLogger(obsvbrutal.WithLevel(obsvbrutal.InfoLevel))

	t.Run("Response Building", func(t *testing.T) {
		router := gin.New()
		router.Use(obsvbrutal.GinMiddleware(logger))

		// Test handler
		router.GET("/test", func(c *gin.Context) {
			log := obsvbrutal.GetLogFrmGin(c, "ResponseTest")
			defer log.Close()

			// สร้าง response builder
			log.R(200)

			// Note: SimpleResponseBuilder จาก GinLogger มี methods จำกัด
			// ไม่มี AddField หรือ Msg methods

			// Note: ใน implementation จริง อาจมี method Send()
			// แต่ตอนนี้ใช้ response options pattern
		})

		// Test request
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("Error Response", func(t *testing.T) {
		router := gin.New()
		router.Use(obsvbrutal.GinMiddleware(logger))

		router.GET("/error", func(c *gin.Context) {
			log := obsvbrutal.GetLogFrmGin(c, "ErrorResponseTest")
			defer log.Close()

			// Error response
			log.R(500)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/error", nil)
		router.ServeHTTP(w, req)

		if w.Code != 500 {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})
}

// Benchmark Trace interface
func BenchmarkTraceInterface(b *testing.B) {
	gin.SetMode(gin.TestMode)
	logger, _ := obsvbrutal.NewLogger(obsvbrutal.WithLevel(obsvbrutal.InfoLevel))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	obsvbrutal.GinMiddleware(logger)(c)

	log := obsvbrutal.GetLogFrmGin(c, "Benchmark")
	defer log.Close()

	b.Run("Simple Trace", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trace := log.FlatPr("benchmark.trace")
			trace.End()
		}
	})

	b.Run("Trace With Attributes", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trace := log.FlatPr("benchmark.trace")
			trace.Add(
				trace.Str("operation", "test"),
				trace.Num("index", float64(i)),
				trace.Bool("success", true),
			)
			trace.End()
		}
	})

	b.Run("Nested Traces", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			parent := log.FlatPr("parent")
			child := parent.FlatPr("child")
			child.End()
			parent.End()
		}
	})
}

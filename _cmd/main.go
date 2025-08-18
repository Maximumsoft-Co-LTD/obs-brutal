package main

import (
	"fmt"
	"net/http"
	"time"

	obsv "github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1) Logger + sinks
	stdout := obsv.NewStdoutSink()
	logger, _ := obsv.NewLogger(obsv.WithLevel(obsv.InfoLevel), obsv.WithSinks(stdout))

	// 2) สร้าง Gin engine พร้อม middleware
	r := obsv.NewGinEngine()
	r.Use(obsv.GinMiddleware(logger))

	// 3) เพิ่ม Prometheus metrics middleware (ถ้าต้องการ)
	// metrics := obsv.NewSafePrometheus("obs-brutal-demo", "development")
	// r.Use(metrics.HandlerFunc())

	// 4) เพิ่ม metrics endpoint (ถ้าต้องการ)
	// r.GET("/metrics", gin.WrapH(obsv.PrometheusHandler()))

	// 5) demo routes
	r.GET("/ping", func(c *gin.Context) {
		// ดึง logger จาก Gin context
		lg := obsv.GetLogFrmGin(c, "PingHandler")
		defer lg.Close()

		// สร้าง trace สำหรับ operation นี้
		tracer := lg.FlatPr("ping.operation")
		defer tracer.End()

		// เพิ่ม attributes ให้ trace
		tracer.Add(
			tracer.Str("endpoint", "/ping"),
			tracer.Str("method", c.Request.Method),
			tracer.Str("client_ip", c.ClientIP()),
			tracer.Num("timestamp", float64(time.Now().Unix())),
		)

		// Log ข้อมูล request
		lg.F("endpoint", "/ping").
			F("method", "GET").
			F("user_agent", c.GetHeader("User-Agent")).
			Prt("Ping request received")

		// ส่ง response
		response := gin.H{
			"ok":        "pong",
			"timestamp": time.Now().Unix(),
			"server":    "obs-brutal-demo",
		}

		// Log response และส่งกลับ
		lg.R(http.StatusOK)
		c.JSON(http.StatusOK, response)
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		lg := obsv.GetLogFrmGin(c, "HealthCheck")
		defer lg.Close()

		lg.Prt("Health check requested")
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Demo endpoint with error handling
	r.GET("/error", func(c *gin.Context) {
		lg := obsv.GetLogFrmGin(c, "ErrorDemo")
		defer lg.Close()

		tracer := lg.FlatPr("error.demo")
		defer tracer.End()

		// จำลอง error
		err := fmt.Errorf("demo error for testing")
		tracer.Err(err)

		lg.Err(err)
		lg.R(http.StatusInternalServerError)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Demo error",
			"code":  500,
		})
	})

	fmt.Println("🚀 obs-brutal demo server starting...")
	fmt.Println("📍 Endpoints:")
	fmt.Println("   GET /ping      - Simple ping endpoint")
	fmt.Println("   GET /health    - Health check")
	fmt.Println("   GET /error     - Error demo")
	fmt.Println("   GET /metrics   - Prometheus metrics")
	fmt.Println("🌐 Server running on http://localhost:8080")

	_ = r.Run(":8080")
}

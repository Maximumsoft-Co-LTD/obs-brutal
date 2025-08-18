package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/app/config"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"

	"github.com/gin-gonic/gin"
)

func main() {
	// Parse command line flags
	var configPath string
	flag.StringVar(&configPath, "c", "", "Path to config file (YAML or .env)")
	flag.Parse()

	// Determine config file to use
	if configPath == "" {
		// Check for default files
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else if _, err := os.Stat(".env"); err == nil {
			configPath = ".env"
		}
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config from file: %v", err)
		// Continue with defaults and env vars
		cfg, _ = config.Load("")
	}

	// Initialize logger [[memory:6403444]]
	base, err := logbrutal.NewLogger(
		logbrutal.WithLevel(logbrutal.ParseLevel(cfg.Log.LEVEL)),
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	logger := base.
		F("service", cfg.AppSrv.SERVICE).
		F("env", cfg.AppSrv.ENVIRONMENT)
	logger.Info("Starting application")
	logger.F("config_file", configPath).Info("Configuration loaded")

	// Initialize OpenTelemetry (optional, graceful degradation)
	var provider *logbrutal.OTelProvider
	if cfg.Otel.ENABLED {
		provider, err = config.SafeInitOTel(
			cfg.AppSrv.SERVICE,
			cfg.Otel.ENDPOINT,
			cfg.Otel.INSECURE,
		)
		if err != nil {
			logger.Err(err).Warn("OpenTelemetry initialization failed, continuing without tracing")
		}
	} else {
		logger.Info("OpenTelemetry disabled in configuration")
	}

	// Setup Gin
	gin.SetMode(cfg.AppSrv.GIN_MODE)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logbrutal.GinMiddleware(logger))

	// Store provider in context if available
	if provider != nil {
		r.Use(func(c *gin.Context) {
			c.Set("otel_provider", provider)
			c.Next()
		})
	}

	// Register routes
	setupRoutes(r, logger, provider)

	// Start metrics server if enabled
	if cfg.Metrics.ENABLED {
		go startMetricsServer(cfg.Metrics.PORT, cfg.Metrics.PATH, logger)
	}

	// Start HTTP server
	srv := &http.Server{
		Addr:    cfg.AppSrv.PORT,
		Handler: r,
	}

	go func() {
		logger.F("addr", srv.Addr).Info("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Err(err).F("addr", srv.Addr).Fatal("HTTP server error")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if provider != nil {
		if err := provider.Shutdown(ctx); err != nil {
			logger.Err(err).Error("Failed to shutdown OpenTelemetry")
		}
	}

	if err := srv.Shutdown(ctx); err != nil {
		logger.Err(err).Error("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

func setupRoutes(r *gin.Engine, logger logbrutal.Logger, provider *logbrutal.OTelProvider) {
	// Health endpoints
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Unix(),
		})
	})

	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"time":   time.Now().Unix(),
		})
	})

	// example.RegisterSimpleDemo() // commented out - not available in standalone

	// API routes
	api := r.Group("/api/v1")
	{
		// Nested tracing demo (hierarchical structure)
		api.GET("/nested", func(c *gin.Context) {
			tracer := logbrutal.GetLogFrmGin(c, "api.nested.demo")
			defer tracer.Close()

			tracer.Prt("Starting nested trace operation")

			// Create child span
			parentSpan := tracer.ChildPr("parent_operation")
			parentSpan.Detail("Parent operation started")

			// Create nested child span
			childSpan := parentSpan.ChildPr("child_operation")
			childSpan.Add(
				childSpan.Str("level", "child"),
				childSpan.Str("type", "nested"),
			)
			childSpan.Detail("Child operation processing")
			simulateWork()

			// Create grandchild span
			grandchildSpan := childSpan.ChildPr("grandchild_operation")
			grandchildSpan.Add(
				grandchildSpan.Str("level", "grandchild"),
				grandchildSpan.Num("depth", 3),
			)
			grandchildSpan.Detail("Deep nested operation")
			simulateWork()
			grandchildSpan.End()

			childSpan.End()
			parentSpan.End()

			c.JSON(http.StatusOK, gin.H{
				"message":      "Nested trace completed",
				"structure":    "hierarchical",
				"trace_id":     tracer.GetTraceID(),
				"span_id":      tracer.GetSpanID(),
				"otel_enabled": provider != nil,
			})
		})

		// Flat tracing demo (sibling structure)
		api.GET("/flat", func(c *gin.Context) {
			tracer := logbrutal.GetLogFrmGin(c, "api.flat.demo")
			defer tracer.Close()

			tracer.Prt("Starting flat trace operation")

			// Create flat spans (all siblings)
			span1 := tracer.FlatPr("operation_1")
			span1.Add(span1.Str("type", "flat"), span1.Num("order", 1))
			span1.Detail("First operation")
			simulateWork()
			span1.End()

			span2 := tracer.FlatPr("operation_2")
			span2.Add(span2.Str("type", "flat"), span2.Num("order", 2))
			span2.Detail("Second operation")
			simulateWork()
			span2.End()

			span3 := tracer.FlatPr("operation_3")
			span3.Add(span3.Str("type", "flat"), span3.Num("order", 3))
			span3.Detail("Third operation")
			simulateWork()
			span3.End()

			c.JSON(http.StatusOK, gin.H{
				"message":      "Flat trace completed",
				"structure":    "flat/sibling",
				"trace_id":     tracer.GetTraceID(),
				"span_id":      tracer.GetSpanID(),
				"otel_enabled": provider != nil,
			})
		})

		// Demo logging with tracing
		api.GET("/log", func(c *gin.Context) {
			// Use GetLogFrmGin for automatic tracing
			tracer := logbrutal.GetLogFrmGin(c, "api.log")
			defer tracer.Close()

			// Log with tracing
			tracer.Prt("Demo log from traced context")
			tracer.F("user_id", 123).Prt("Log with field")

			// Create child span for processing
			childSpan := tracer.FlatPr("process_log_request")
			childSpan.Add(
				childSpan.Str("endpoint", "/api/v1/log"),
				childSpan.Str("method", c.Request.Method),
				childSpan.Str("client_ip", c.ClientIP()),
			)
			childSpan.Detail("Processing log request")

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			childSpan.End()

			c.JSON(http.StatusOK, gin.H{
				"message":  "Log generated with tracing",
				"trace_id": tracer.GetTraceID(),
				"span_id":  tracer.GetSpanID(),
			})
		})

		// Error handling demo
		api.GET("/error", func(c *gin.Context) {
			// Simulate an error
			err := processOrder("invalid-order")
			if err != nil {
				logger.
					Err(err).
					F("endpoint", "/api/v1/error").
					Error("Failed to process order")

				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Success",
			})
		})

		// Structured logging demo
		api.POST("/order", func(c *gin.Context) {
			var order struct {
				ID     string  `json:"id"`
				UserID string  `json:"user_id"`
				Amount float64 `json:"amount"`
				Items  []struct {
					Name  string  `json:"name"`
					Price float64 `json:"price"`
					Qty   int     `json:"qty"`
				} `json:"items"`
			}

			if err := c.ShouldBindJSON(&order); err != nil {
				logger.Err(err).Error("Invalid order data")
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Invalid request body",
				})
				return
			}

			// Log with structured data
			orderLogger := logger.
				F("order_id", order.ID).
				F("user_id", order.UserID).
				F("amount", order.Amount).
				F("item_count", len(order.Items))

			orderLogger.Info("Processing order")

			// Simulate processing
			time.Sleep(100 * time.Millisecond)

			// Log completion
			orderLogger.
				F("processing_time_ms", 100).
				Info("Order processed successfully")

			c.JSON(http.StatusOK, gin.H{
				"order_id": order.ID,
				"status":   "processed",
			})
		})

		// Tracing demo
		api.GET("/trace", func(c *gin.Context) {
			// Use GetLogFrmGin for tracing (works even without provider)
			tracer := logbrutal.GetLogFrmGin(c, "api.trace.demo")
			defer tracer.Close()

			tracer.Prt("Starting trace operation")

			// Create child span for fetching data
			fetchSpan := tracer.ChildPr("fetch_data")
			fetchSpan.Add(
				fetchSpan.Str("operation", "fetch_datatch"),
				fetchSpan.Str("source", "database"),
			)
			fetchSpan.Detail("Fetching user data from database")
			simulateWork()
			fetchSpan.End()

			// Create flat span for processing (sibling to fetch_data)
			processSpan := tracer.FlatPr("process_data")
			processSpan.Add(
				processSpan.Str("operation", "transform"),
				processSpan.Num("records", 100),
			)
			processSpan.Detail("Processing fetched data")
			simulateWork()
			processSpan.End()

			// Create flat span for saving (sibling to others)
			saveSpan := tracer.FlatPr("save_data")
			saveSpan.Add(
				saveSpan.Str("operation", "save"),
				saveSpan.Str("destination", "cache"),
			)
			saveSpan.Detail("Saving processed data to cache")
			simulateWork()
			saveSpan.End()

			var opts = logbrutal.OptsResponse()
			tracer.R(http.StatusOK, opts.Msg("Trace operation completed"), opts.Response(gin.H{
				"trace_id":     tracer.GetTraceID(),
				"span_id":      tracer.GetSpanID(),
				"otel_enabled": provider != nil,
			}))
		})
	}

	// Demo group
	demo := r.Group("/demo")
	{
		// Performance metrics
		demo.GET("/metrics", func(c *gin.Context) {
			start := time.Now()
			defer func() {
				logger.
					F("duration_ms", time.Since(start).Milliseconds()).
					F("path", c.Request.URL.Path).
					Info("Request completed")
			}()

			// Simulate some work
			simulateWork()

			c.JSON(http.StatusOK, gin.H{
				"metrics": map[string]interface{}{
					"requests":    1000,
					"errors":      10,
					"success":     990,
					"uptime_secs": time.Since(start).Seconds(),
				},
			})
		})

		// Test different log levels
		demo.GET("/levels", func(c *gin.Context) {
			logger.Debug("This is a debug message")
			logger.Info("This is an info message")
			logger.Warn("This is a warning message")
			logger.Error("This is an error message")

			c.JSON(http.StatusOK, gin.H{
				"message": "Logged messages at different levels",
			})
		})

		// Field combinations
		demo.GET("/fields", func(c *gin.Context) {
			// Chain multiple fields
			logger.
				F("string", "value").
				F("number", 123).
				F("float", 45.67).
				F("bool", true).
				F("array", []int{1, 2, 3}).
				F("map", map[string]interface{}{
					"key1": "value1",
					"key2": 2,
				}).
				Info("Log with various field types")

			// Use specific field helpers
			logger.
				UID("user-123").
				RID("req-456").
				TID("trace-789").
				SID("span-abc").
				Tenant("tenant-xyz").
				Info("Log with specific fields")

			c.JSON(http.StatusOK, gin.H{
				"message": "Logged with various fields",
			})
		})
	}
}

func startMetricsServer(port int, path string, logger logbrutal.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		// In a real app, you would expose actual metrics here
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# HELP log_brutal_example Example metrics\n"))
		w.Write([]byte("# TYPE log_brutal_example counter\n"))
		w.Write([]byte("log_brutal_example 1\n"))
	})

	addr := fmt.Sprintf(":%d", port)
	logger.F("addr", addr).F("path", path).Info("Starting metrics server")

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Err(err).F("addr", addr).Error("Metrics server error")
	}
}

func processOrder(orderID string) error {
	if orderID == "invalid-order" {
		return fmt.Errorf("invalid order ID: %s", orderID)
	}
	return nil
}

func simulateWork() {
	time.Sleep(50 * time.Millisecond)
}

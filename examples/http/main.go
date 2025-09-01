package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "obs-brutal/internal/util"
    "obs-brutal/logtrc"

    "github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	// /metrics (Prometheus) via OTEL provider
    if _, prov, err := logtrc.NewOTelWithService("example-http", "1.0.0", "dev", "localhost:4317", logtrc.INFO); err == nil {
        // ลงทะเบียน /metrics ด้วย Prometheus handler
        r.GET("/metrics", gin.WrapH(prov.PrometheusHandler()))
    }

	// Basic middleware attaching a per-request logbrut
	r.Use(logtrc.Middleware("example-http"))

    r.GET("/health", func(c *gin.Context) {
        // demonstrate type-safe context keys
        ctx := util.WithTraceID(c.Request.Context(), "demo-trace-id")
        log := logtrc.GetLog(c).Ctx(ctx)
        log.F("environment", "dev").Info("health check")
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    })

	r.GET("/users/:id", func(c *gin.Context) {
		log := logtrc.GetLog(c).F("route", "/users/:id")
		id := c.Param("id")
		// simulate lookup
		time.Sleep(20 * time.Millisecond)
		log.F("user_id", id).Info("fetch user")
		c.JSON(http.StatusOK, gin.H{"id": id, "name": "demo"})
	})

	r.POST("/orders", func(c *gin.Context) {
		ltrace := logtrc.GetLogTrcFrmGin(c, "create_order")
		// pretend to process
		time.Sleep(15 * time.Millisecond)
		ltrace.Prt("order created")
		ltrace.R(http.StatusCreated, logtrc.Opts.Msg("created"), logtrc.Opts.Body(gin.H{"order_id": "o_123"})).Send()
	})

	srv := &http.Server{Addr: ":8081", Handler: r}
	go func() { _ = srv.ListenAndServe() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
    // Provider shutdown is handled by adapter user if needed
}

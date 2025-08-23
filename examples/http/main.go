package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"obs-brutal/logtrc"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	// /metrics (Prometheus) via OTEL provider
	var otel *logtrc.OTelLogBrt
	if o, err := logtrc.NewOTelLogBrt("example-http", "1.0.0", "dev", "localhost:4317", logtrc.INFO); err == nil {
		otel = o
		// ลงทะเบียน /metrics ด้วย Prometheus handler
		r.GET("/metrics", gin.WrapH(otel.GetOTelProvider().PrometheusHandler()))
	}

	// Basic middleware attaching a per-request logger
	r.Use(logtrc.Middleware("example-http"))

	r.GET("/health", func(c *gin.Context) {
		log := logtrc.GetLog(c)
		log.Info("health check")
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
	if otel != nil {
		_ = otel.GetOTelProvider().Shutdown(ctx)
	}
}

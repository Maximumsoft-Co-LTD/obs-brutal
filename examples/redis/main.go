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
    "github.com/redis/go-redis/v9"
)

func main() {
	r := gin.New()
	r.Use(logtrc.Middleware("example-redis"))

	// Real go-redis v9 client
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 0})
	defer rc.Close()

    r.GET("/cache/:key", func(c *gin.Context) {
        ctx := util.WithTraceID(c.Request.Context(), "redis-trace-001")
        log := logtrc.GetLog(c).Ctx(ctx)
        key := c.Param("key")
        val, err := rc.Get(ctx, key).Result()
		if err == redis.Nil {
			log.F("key", key).Info("cache miss")
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			log.WithError(err).Error("redis get failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
        log.F("environment", "dev").F("key", key).Info("cache hit")
		c.JSON(http.StatusOK, gin.H{"key": key, "value": val})
	})

    r.POST("/cache/:key", func(c *gin.Context) {
        ctx := util.WithTraceID(c.Request.Context(), "redis-trace-001")
        key := c.Param("key")
		val := c.Query("v")
		if val == "" {
			val = "1"
		}
		if err := rc.Set(ctx, key, val, 10*time.Minute).Err(); err != nil {
			logtrc.GetLog(c).WithError(err).Error("redis set failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
        logtrc.GetLog(c).Ctx(ctx).F("environment", "dev").F("key", key).Info("saved")
		c.JSON(http.StatusOK, gin.H{"ok": true, "stored": val})
	})

	srv := &http.Server{Addr: ":8082", Handler: r}
	go func() { _ = srv.ListenAndServe() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

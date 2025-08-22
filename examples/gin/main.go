package main

import (
	"net/http"
	"os"

	"obs-brutal/logtrc"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()

	// Read endpoint from env (fallback to jaeger:4317)
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = "jaeger:4317"
	}

	// Create OTEL-enabled LogBrt and middleware
	otelLogBrt, err := logtrc.NewOTelLogBrt("example-gin", "1.0.0", "dev", endpoint, logtrc.INFO)
	if err == nil && otelLogBrt != nil {
		r.Use(logtrc.OTelMiddleware(otelLogBrt))
	} else {
		// Fallback to basic middleware if OTEL not available
		r.Use(logtrc.Middleware("example-gin"))
	}

	r.GET("/", func(c *gin.Context) {
		// Prefer OTEL-aware LogBrt if present
		log := logtrc.GetOTelLog(c)
		log.Info("hello from examples/gin (otel)")
		c.String(http.StatusOK, "ok")
	})

	r.Run(":8080")
}

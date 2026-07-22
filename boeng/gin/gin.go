// Package boenggin instruments github.com/gin-gonic/gin handlers with
// boeng's Operation Runtime. Each request becomes a boeng operation;
// W3C trace context is extracted from incoming request headers so the
// request joins the caller's distributed trace.
//
// Business handlers never import OpenTelemetry — boeng.L(c.Request.Context())
// returns the request-scoped logger, and any boeng.Run / boeng.Step calls
// inside the handler nest under the request's operation automatically.
package boenggin

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// Middleware returns a Gin middleware that opens a boeng operation per
// request. The operation is named "<METHOD> <route>". Trace context from
// the incoming request headers is honored so this op nests under the
// upstream caller's span when one is propagated.
//
//	r := gin.New()
//	r.Use(boenggin.Middleware())
//	r.GET("/users/:id", func(c *gin.Context) {
//	    boenggin.L(c).Info("hello")
//	})
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)

		// FullPath() is the low-cardinality route template (e.g.
		// "/users/:id"). On an unmatched route it is empty; fall back to
		// the raw path so the op is still identifiable. Metric-name
		// cardinality is bounded downstream by the fail-closed cap in
		// boeng's metrics layer.
		name := c.Request.Method + " " + c.FullPath()
		if c.FullPath() == "" {
			name = c.Request.Method + " " + c.Request.URL.Path
		}

		_ = boeng.Run(ctx, name, requestSubject(c), func(opCtx context.Context) error {
			c.Request = c.Request.WithContext(opCtx)
			c.Next()
			if len(c.Errors) > 0 {
				return c.Errors.Last().Err
			}
			return nil
		})
	}
}

// L returns the request-scoped boeng logger from gin.Context.
//
//	func handler(c *gin.Context) {
//	    boenggin.L(c).Info("got request")
//	}
func L(c *gin.Context) boeng.Logger {
	return boeng.L(c.Request.Context())
}

// requestFields produces the per-request log/span fields. The keys
// follow the OpenTelemetry semantic-conventions vocabulary so downstream
// dashboards can be reused across services.
type requestFields struct {
	Method     string
	Route      string
	Path       string
	RemoteAddr string
}

func (r requestFields) LogFields() map[string]any {
	return map[string]any{
		"http.method": r.Method,
		"http.route":  r.Route,
		"http.path":   r.Path,
		"http.remote": r.RemoteAddr,
	}
}

func requestSubject(c *gin.Context) any {
	return requestFields{
		Method:     c.Request.Method,
		Route:      c.FullPath(),
		Path:       c.Request.URL.Path,
		RemoteAddr: c.ClientIP(),
	}
}

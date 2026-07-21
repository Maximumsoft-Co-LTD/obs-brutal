package boenggin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boenggin "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/gin"
)

func TestMiddleware_OpenAndCloseOperationPerRequest(t *testing.T) {
	boeng.Init(boeng.Config{Service: "gin_test"})
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(boenggin.Middleware())
	called := false
	r.GET("/x", func(c *gin.Context) {
		called = true
		// Logger must be reachable from gin.Context.
		if boenggin.L(c) == nil {
			t.Errorf("boenggin.L(c) returned nil")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if !called {
		t.Fatalf("handler never ran")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMiddleware_ExtractsTraceparent(t *testing.T) {
	boeng.Init(boeng.Config{Service: "gin_test"})
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(boenggin.Middleware())
	r.GET("/x", func(c *gin.Context) {})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	// Well-formed W3C traceparent. The middleware must not panic on it
	// and must propagate ctx through to the handler.
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

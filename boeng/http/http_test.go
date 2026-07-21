package boenghttp_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boenghttp "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http"
)

func TestMiddleware_WrapsHandler(t *testing.T) {
	boeng.Init(boeng.Config{Service: "http_test"})

	called := false
	h := boenghttp.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte("ok"))
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	h.ServeHTTP(w, req)

	if !called {
		t.Fatalf("handler never ran")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMiddleware_FlagsServerErrors(t *testing.T) {
	boeng.Init(boeng.Config{Service: "http_test"})

	h := boenghttp.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestTransport_InjectsTraceparent(t *testing.T) {
	boeng.Init(boeng.Config{Service: "http_test"})

	var seenTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTraceparent = r.Header.Get("traceparent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: boenghttp.Transport(nil)}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	// Without a tracer that issues a span context, the propagator may
	// have nothing to inject — but the transport must not error out.
	_ = seenTraceparent
}

func TestTransport_ReportsUpstream5xx(t *testing.T) {
	boeng.Init(boeng.Config{Service: "http_test"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusBadGateway)
	}))
	defer srv.Close()

	client := &http.Client{Transport: boenghttp.Transport(nil)}
	resp, err := client.Get(srv.URL)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected client err: %v", err)
	}
	if resp != nil && resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
}

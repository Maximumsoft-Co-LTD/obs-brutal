// Package boenghttp instruments net/http handlers and clients with
// boeng's Operation Runtime.
//
// Server side: Middleware opens a boeng operation per request and
// extracts W3C trace context from incoming headers so the request
// joins the caller's distributed trace.
//
// Client side: Transport wraps an http.RoundTripper and injects W3C
// trace headers into outgoing requests so downstream services can
// continue the trace.
//
// Business code never imports OpenTelemetry — adapters do the
// extraction/injection internally.
package boenghttp

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// Middleware wraps an http.Handler so each request becomes a boeng
// operation named "<METHOD> <path>". W3C trace context from incoming
// request headers is extracted and propagated.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(
			r.Context(),
			propagation.HeaderCarrier(r.Header),
		)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		name := r.Method + " " + r.URL.Path
		_ = boeng.Run(ctx, name, requestSubject(r), func(opCtx context.Context) error {
			next.ServeHTTP(sw, r.WithContext(opCtx))
			if sw.status >= 500 {
				return errors.New(http.StatusText(sw.status))
			}
			return nil
		})
	})
}

// Wrap is a convenience helper that turns a plain http.HandlerFunc into
// a boeng-instrumented handler.
//
//	mux.Handle("/users", boenghttp.Wrap("list_users", listUsers))
func Wrap(name string, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(
			r.Context(),
			propagation.HeaderCarrier(r.Header),
		)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		_ = boeng.Run(ctx, name, requestSubject(r), func(opCtx context.Context) error {
			fn(sw, r.WithContext(opCtx))
			if sw.status >= 500 {
				return errors.New(http.StatusText(sw.status))
			}
			return nil
		})
	}
}

// Transport returns an http.RoundTripper that wraps inner with a boeng
// operation per outgoing request AND injects W3C trace context into
// the request headers so downstream services can join the trace.
//
//	client := &http.Client{Transport: boenghttp.Transport(http.DefaultTransport)}
//
// If inner is nil, http.DefaultTransport is used.
func Transport(inner http.RoundTripper) http.RoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &transport{inner: inner}
}

type transport struct {
	inner http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	ctx := req.Context()
	name := "HTTP " + req.Method + " " + req.URL.Host
	// Run records 5xx into the boeng operation as a failure (ERROR log
	// + _error_total bump) but we deliberately do NOT propagate that as
	// the RoundTripper's error: per Go's net/http contract, a 5xx is a
	// successful transport (server responded) and must be returned in
	// resp, not err. Returning both violates the contract.
	_ = boeng.Run(ctx, name, outgoingSubject(req), func(opCtx context.Context) error {
		otel.GetTextMapPropagator().Inject(opCtx, propagation.HeaderCarrier(req.Header))
		resp, err = t.inner.RoundTrip(req.WithContext(opCtx))
		if err != nil {
			return err
		}
		if resp.StatusCode >= 500 {
			return errors.New("upstream " + resp.Status)
		}
		return nil
	})
	return resp, err
}

// statusWriter remembers the response code so the middleware can flip
// the boeng operation to ERROR on 5xx without forcing handlers to
// return an error themselves.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}

type requestFields struct {
	Method     string
	Path       string
	RemoteAddr string
}

func (r requestFields) LogFields() map[string]any {
	return map[string]any{
		"http.method": r.Method,
		"http.path":   r.Path,
		"http.remote": r.RemoteAddr,
	}
}

func requestSubject(r *http.Request) any {
	return requestFields{
		Method:     r.Method,
		Path:       r.URL.Path,
		RemoteAddr: r.RemoteAddr,
	}
}

type outgoingFields struct {
	Method string
	URL    string
}

func (o outgoingFields) LogFields() map[string]any {
	return map[string]any{
		"http.method": o.Method,
		"http.url":    o.URL,
	}
}

func outgoingSubject(r *http.Request) any {
	return outgoingFields{Method: r.Method, URL: r.URL.String()}
}

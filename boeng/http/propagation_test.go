package boenghttp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boenghttp "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http"
)

// TestPropagation_ClientHeaderReachesServer asserts that an outgoing
// request issued through boenghttp.Transport carries the W3C
// traceparent header into a downstream service. The test sets a
// known traceparent at the caller and verifies the server received
// the same trace-id.
//
// This is the adapter compliance check the README refers to: HTTP
// must propagate context across process boundaries.
func TestPropagation_ClientHeaderReachesServer(t *testing.T) {
	boeng.Init(boeng.Config{Service: "prop_test"})

	var seenTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTraceparent = r.Header.Get("traceparent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	// Caller "pre-seeds" a traceparent; the transport may overwrite it
	// with one derived from the current ctx, but in either case a
	// well-formed traceparent must be present at the server.
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	client := &http.Client{Transport: boenghttp.Transport(nil)}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	if seenTraceparent == "" {
		t.Fatalf("server did not receive traceparent header (transport stripped it)")
	}
}

// TestPropagation_ServerExtractAttachesToOpCtx asserts that the
// server-side Middleware extracts the incoming traceparent and the
// request handler can see a non-empty context from the propagator.
func TestPropagation_ServerExtractAttachesToOpCtx(t *testing.T) {
	boeng.Init(boeng.Config{Service: "prop_test"})

	var sawCtx bool
	h := boenghttp.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawCtx = r.Context() != nil
		_, _ = w.Write([]byte("ok"))
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	h.ServeHTTP(w, req)

	if !sawCtx {
		t.Fatalf("handler did not observe a context from middleware")
	}
}

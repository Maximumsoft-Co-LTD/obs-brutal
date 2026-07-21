// net/http adapter demo. Shows BOTH the server middleware and the
// client RoundTripper. The client makes a request to the same server,
// which means the trace context is propagated end-to-end through the
// HTTP headers (W3C traceparent).
//
// Run: go run ./examples/http
//
// The server log lines and the client log lines for one round-trip will
// share the same trace_id.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boenghttp "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http"
)

func main() {
	defer boeng.Init(boeng.Config{Service: "http_demo", Env: "dev"}).Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/users/", boenghttp.Wrap("get_user", func(w http.ResponseWriter, r *http.Request) {
		boeng.L(r.Context()).Info("inside handler")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	server := &http.Server{Addr: ":8081", Handler: boenghttp.Middleware(mux)}
	go func() { _ = server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(200 * time.Millisecond)

	client := &http.Client{Transport: boenghttp.Transport(nil)}
	resp, err := client.Get("http://localhost:8081/users/u-7")
	if err != nil {
		fmt.Println("client err:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("client got:", string(body))
}

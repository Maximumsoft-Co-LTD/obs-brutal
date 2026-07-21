package network

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"

    "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func TestLokiBatchSizeFlush(t *testing.T) {
    var got int64
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer r.Body.Close()
        var payload struct{ Streams []struct{ Values [][]string `json:"values"` } `json:"streams"` }
        if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
            t.Fatalf("decode: %v", err)
        }
        atomic.AddInt64(&got, int64(len(payload.Streams[0].Values)))
        w.WriteHeader(204)
    }))
    defer srv.Close()

    s := NewLokiPushSink(srv.URL, map[string]string{"app":"t"}).(*LokiPushSink)
    _ = s.Configure(map[string]interface{}{"batch_size": 2})

    e1 := &domain.LogEntry{Timestamp: time.Now()}
    e2 := &domain.LogEntry{Timestamp: time.Now()}
    if err := s.Write(e1); err != nil { t.Fatal(err) }
    if err := s.Write(e2); err != nil { t.Fatal(err) }

    // size reached → immediate flush; wait briefly
    time.Sleep(50 * time.Millisecond)
    if n := atomic.LoadInt64(&got); n != 2 {
        t.Fatalf("expected 2 values in one batch, got %d", n)
    }
    _ = s.Close()
}

func TestLokiRetry(t *testing.T) {
    var calls int64
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer r.Body.Close()
        c := atomic.AddInt64(&calls, 1)
        if c == 1 { w.WriteHeader(500); return }
        w.WriteHeader(204)
    }))
    defer srv.Close()

    s := NewLokiPushSink(srv.URL, nil).(*LokiPushSink)
    _ = s.Configure(map[string]interface{}{"batch_size": 1, "max_retries": 2, "retry_base_ms": 10})

    e := &domain.LogEntry{Timestamp: time.Now()}
    if err := s.Write(e); err != nil { t.Fatalf("write: %v", err) }
    // write triggers flush immediately (batch_size=1)
    time.Sleep(50 * time.Millisecond)
    if n := atomic.LoadInt64(&calls); n < 2 {
        t.Fatalf("expected retry, calls=%d", n)
    }
    _ = s.Close()
}


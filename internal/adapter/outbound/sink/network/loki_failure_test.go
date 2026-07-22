package network

import (
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

// deadSink returns a LokiPushSink pointed at a port nothing listens on
// (connection refused immediately) with retries tuned down so tests
// stay fast.
func deadSink(t *testing.T, batch int) *LokiPushSink {
	t.Helper()
	s := NewLokiPushSink("http://127.0.0.1:1/loki/api/v1/push", map[string]string{"app": "t"}).(*LokiPushSink)
	if err := s.Configure(map[string]interface{}{
		"batch_size":    batch,
		"max_retries":   0,
		"retry_base_ms": 1,
		"timeout_ms":    3600000, // background ticker out of the way
	}); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func entry(msg string) *domain.LogEntry {
	return &domain.LogEntry{Level: domain.InfoLevel, Msg: msg, Timestamp: time.Now()}
}

// TestLokiPushSink_DownEndpoint_BufferBounded: when Loki is
// unreachable, failed flushes re-queue their batch. Without a cap the
// buffer grows with every write for as long as the outage lasts — an
// unbounded memory leak inside any long-running process. The retry
// buffer must stay bounded.
func TestLokiPushSink_DownEndpoint_BufferBounded(t *testing.T) {
	const batch = 10
	s := deadSink(t, batch)

	for i := 0; i < 1000; i++ {
		_ = s.Write(entry("x"))
	}

	s.mu.Lock()
	buffered := len(s.values)
	s.mu.Unlock()

	// Generous bound: an order of magnitude over the batch size. The
	// exact cap is an implementation detail; unbounded growth is the bug.
	if limit := batch * 20; buffered > limit {
		t.Fatalf("retry buffer grew unbounded during outage: %d entries buffered (want <= %d)", buffered, limit)
	}
}

// TestLokiPushSink_CloseIsIdempotent: Init-twice and defer patterns
// both end up closing sinks more than once; the second Close must be a
// no-op, not a "close of closed channel" panic.
func TestLokiPushSink_CloseIsIdempotent(t *testing.T) {
	s := deadSink(t, 1000)
	if err := s.Close(); err == nil {
		// dead endpoint: final flush fails, error is fine — panic is not
		t.Log("first Close returned nil (nothing buffered)")
	}
	_ = s.Close() // must not panic
}

// TestLokiPushSink_DownEndpoint_CloseReturns: Close performs a final
// flush; with the endpoint down it must still return promptly instead
// of hanging the process shutdown path.
func TestLokiPushSink_DownEndpoint_CloseReturns(t *testing.T) {
	s := deadSink(t, 1000) // batch above what we write: flush only happens at Close
	for i := 0; i < 50; i++ {
		_ = s.Write(entry("pending"))
	}
	done := make(chan struct{})
	go func() { _ = s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung for >5s with unreachable endpoint")
	}
}

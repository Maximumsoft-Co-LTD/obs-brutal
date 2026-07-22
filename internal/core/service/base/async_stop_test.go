package base

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

type countingSink struct {
	mu      sync.Mutex
	entries []*domain.LogEntry
}

func (c *countingSink) Write(e *domain.LogEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, e)
	return nil
}
func (c *countingSink) Close() error                            { return nil }
func (c *countingSink) Name() string                            { return "counting" }
func (c *countingSink) Health() error                           { return nil }
func (c *countingSink) Configure(map[string]interface{}) error  { return nil }

func (c *countingSink) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// TestAsyncPipeline_StopDrainsPending is the regression test for the
// flush-on-close defect: entries enqueued right before Stop must reach
// the sink. The flush timeout is deliberately much longer than the test
// so only the drain path can deliver them.
func TestAsyncPipeline_StopDrainsPending(t *testing.T) {
	sink := &countingSink{}
	ap := NewAsyncPipeline(1000, 4, time.Hour, sink)

	const n = 50
	for i := 0; i < n; i++ {
		if !ap.WriteAsync(&domain.LogEntry{Msg: fmt.Sprintf("m%d", i), Timestamp: time.Now()}) {
			t.Fatalf("entry %d rejected before Stop", i)
		}
	}
	ap.Stop()

	if got := sink.count(); got != n {
		t.Fatalf("Stop dropped pending entries: sink got %d of %d", got, n)
	}
}

// TestAsyncPipeline_WriteAfterStopRejected documents the post-Stop
// contract: WriteAsync must return false so callers fall back to their
// synchronous write path instead of enqueueing into a dead pipeline.
func TestAsyncPipeline_WriteAfterStopRejected(t *testing.T) {
	ap := NewAsyncPipeline(10, 1, time.Hour, &countingSink{})
	ap.Stop()
	if ap.WriteAsync(&domain.LogEntry{Msg: "late", Timestamp: time.Now()}) {
		t.Fatal("WriteAsync accepted an entry after Stop")
	}
	// Stop must stay idempotent.
	ap.Stop()
}

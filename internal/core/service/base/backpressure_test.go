package base

import (
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

// blockingSink blocks every Write until release is closed, letting the
// test wedge the whole pipeline (sink worker busy → batchChan full →
// batch worker blocked → logChan full) deterministically.
type blockingSink struct{ release chan struct{} }

func (b *blockingSink) Write(*domain.LogEntry) error {
	<-b.release
	return nil
}
func (b *blockingSink) Close() error                           { return nil }
func (b *blockingSink) Name() string                           { return "blocking" }
func (b *blockingSink) Health() error                          { return nil }
func (b *blockingSink) Configure(map[string]interface{}) error { return nil }

// TestAsync_BlockShort verifies the documented BlockShort policy: when
// the queue is full, WriteAsync blocks up to blockFor and then drops —
// it must neither drop instantly (that's DropLatest) nor block forever.
func TestAsync_BlockShort(t *testing.T) {
	sink := &blockingSink{release: make(chan struct{})}
	ap := NewAsyncPipeline(1, 1, time.Hour, sink)
	ap.policy.Store(int32(BlockShort))
	ap.blockFor.Store((50 * time.Millisecond).Nanoseconds())

	mk := func() *domain.LogEntry {
		return &domain.LogEntry{Level: domain.InfoLevel, Msg: "x", Timestamp: time.Now()}
	}

	// Fill until the first drop. Capacity is finite (logChan 4 +
	// batchChan 2 + one batch in the blocked worker + one in the sink),
	// so this terminates quickly once everything is wedged.
	var dropped bool
	for i := 0; i < 64; i++ {
		start := time.Now()
		if !ap.WriteAsync(mk()) {
			if waited := time.Since(start); waited < 40*time.Millisecond {
				t.Fatalf("BlockShort dropped after %v; must wait ~blockFor (50ms) first", waited)
			}
			dropped = true
			break
		}
	}
	if !dropped {
		t.Fatal("queue never filled; test setup no longer matches pipeline internals")
	}
	if st := ap.Stats(); st.Dropped != 1 {
		t.Fatalf("expected exactly 1 dropped entry, got %d", st.Dropped)
	}

	close(sink.release) // unwedge so Stop can drain
	ap.Stop()
}

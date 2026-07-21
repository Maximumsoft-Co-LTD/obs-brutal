package base

import (
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

// Test DropOldest policy keeps queue full and counts dropped correctly
func TestAsync_DropOldest(t *testing.T) {
	// batchSize=1 → queue cap = batchSize*4 = 4; no workers; no flush
	ap := NewAsyncPipeline(1, 0, time.Hour)
	ap.policy.Store(int32(DropOldest))

	mk := func(i int) *domain.LogEntry {
		return &domain.LogEntry{Level: domain.InfoLevel, Msg: "x", Timestamp: time.Now()}
	}

	// fill up to capacity (4)
	for i := 0; i < 4; i++ {
		if ok := ap.WriteAsync(mk(i)); !ok {
			t.Fatalf("enqueue %d failed unexpectedly", i)
		}
	}
	// next should drop one oldest and accept new; queue stays at 4
	if ok := ap.WriteAsync(mk(4)); !ok {
		t.Fatalf("expected accept after drop oldest")
	}
	st := ap.Stats()
	if st.QueueSize != 4 {
		t.Fatalf("expected queue size 4, got %d", st.QueueSize)
	}
	if st.Dropped != 1 {
		t.Fatalf("expected dropped=1, got %d", st.Dropped)
	}
	ap.Stop()
}

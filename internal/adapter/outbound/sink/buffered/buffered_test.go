package buffered

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

type failingSink struct {
	mu     sync.Mutex
	writes int
	fail   bool
}

func (f *failingSink) Write(*domain.LogEntry) error {
	f.mu.Lock()
	f.writes++
	f.mu.Unlock()
	if f.fail {
		return errors.New("down")
	}
	return nil
}
func (f *failingSink) Close() error                           { return nil }
func (f *failingSink) Name() string                           { return "failing" }
func (f *failingSink) Health() error                          { return nil }
func (f *failingSink) Configure(map[string]interface{}) error { return nil }
func (f *failingSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

func entry() *domain.LogEntry { return &domain.LogEntry{Msg: "x", Timestamp: time.Now()} }

// TestDoubleClose_NoPanic: Close must be idempotent — a second Close
// (composite-logger shutdown + app defer) must not panic.
func TestDoubleClose_NoPanic(t *testing.T) {
	s := NewBufferedSinkWith(10, time.Hour)
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("second Close panicked: %v", r)
		}
	}()
	_ = s.Close()
}

// TestFlushFailure_RetainsEntriesAndReportsError: when the inner sink is
// down, a size-triggered flush must NOT silently discard the entries and
// must surface the error, instead of returning nil and emptying the
// buffer.
func TestFlushFailure_RetainsEntriesAndReportsError(t *testing.T) {
	inner := &failingSink{fail: true}
	s := NewBuf(inner, 10, time.Hour).(*BufferedSink)

	var lastErr error
	for i := 0; i < 10; i++ {
		lastErr = s.Write(entry())
	}
	// The 10th write triggers a flush; every inner write failed.
	if lastErr == nil {
		t.Fatal("flush with a down inner sink returned nil — failure swallowed")
	}
	s.mu.Lock()
	buffered := len(s.buffer)
	s.mu.Unlock()
	if buffered != 10 {
		t.Fatalf("failed entries were discarded: %d retained (want 10)", buffered)
	}

	// Recovery: once the inner sink is healthy again, the retained
	// entries flush through.
	inner.fail = false
	if err := s.Close(); err != nil {
		t.Fatalf("Close after recovery: %v", err)
	}
	if inner.count() < 20 { // 10 failed attempts + 10 successful on flush
		t.Fatalf("retained entries were not re-flushed after recovery: inner saw %d writes", inner.count())
	}
}

// TestFlushRetainBounded: a prolonged outage must not grow the retry
// buffer without limit.
func TestFlushRetainBounded(t *testing.T) {
	inner := &failingSink{fail: true}
	s := NewBuf(inner, 10, time.Hour).(*BufferedSink)
	for i := 0; i < 5000; i++ {
		_ = s.Write(entry())
	}
	s.mu.Lock()
	buffered := len(s.buffer)
	s.mu.Unlock()
	if buffered > s.retainCap() {
		t.Fatalf("retry buffer unbounded during outage: %d (cap %d)", buffered, s.retainCap())
	}
}

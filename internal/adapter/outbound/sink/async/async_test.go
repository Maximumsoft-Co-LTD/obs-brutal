package async

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

type recordingSink struct {
	mu      sync.Mutex
	writes  int
	closed  atomic.Bool
	slowDur time.Duration
}

func (r *recordingSink) Write(*domain.LogEntry) error {
	if r.slowDur > 0 {
		time.Sleep(r.slowDur)
	}
	r.mu.Lock()
	r.writes++
	r.mu.Unlock()
	return nil
}
func (r *recordingSink) Close() error                           { r.closed.Store(true); return nil }
func (r *recordingSink) Name() string                           { return "recording" }
func (r *recordingSink) Health() error                          { return nil }
func (r *recordingSink) Configure(map[string]interface{}) error { return nil }
func (r *recordingSink) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writes
}

func entry() *domain.LogEntry { return &domain.LogEntry{Msg: "x", Timestamp: time.Now()} }

// TestClose_DrainsQueuedEntries: entries enqueued right before Close must
// all reach the inner sink — Close drains, it does not drop.
func TestClose_DrainsQueuedEntries(t *testing.T) {
	inner := &recordingSink{slowDur: time.Millisecond}
	s := New(inner, 1000, 1)
	const n = 200
	for range n {
		_ = s.Write(entry())
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := inner.count(); got != n {
		t.Fatalf("Close dropped queued entries: inner got %d of %d", got, n)
	}
}

// TestWriteAfterClose_NoPanic: a Write after Close must drop quietly, not
// panic on a send to a closed channel.
func TestWriteAfterClose_NoPanic(t *testing.T) {
	inner := &recordingSink{}
	s := New(inner, 10, 1)
	_ = s.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Write after Close panicked: %v", r)
		}
	}()
	for range 5 {
		if err := s.Write(entry()); err != nil {
			t.Fatalf("Write after Close returned error: %v", err)
		}
	}
}

// TestConcurrentWriteClose_NoPanic: Write racing Close must never panic.
func TestConcurrentWriteClose_NoPanic(t *testing.T) {
	inner := &recordingSink{}
	s := New(inner, 100, 4)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_ = s.Write(entry())
			}
		}()
	}
	time.Sleep(2 * time.Millisecond)
	_ = s.Close()
	wg.Wait()
}

// TestClose_ClosesInner: Close must propagate to the wrapped sink so
// buffering inner sinks flush and release resources.
func TestClose_ClosesInner(t *testing.T) {
	inner := &recordingSink{}
	s := New(inner, 10, 1)
	_ = s.Close()
	if !inner.closed.Load() {
		t.Fatal("async Close did not call inner.Close()")
	}
}

// TestClose_Idempotent: a second Close must be a no-op, not a panic.
func TestClose_Idempotent(t *testing.T) {
	s := New(&recordingSink{}, 10, 1)
	_ = s.Close()
	_ = s.Close()
}

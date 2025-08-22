package core

import (
	"testing"
	"time"
)

func TestBufferedSinkCloseStopsFlusher(t *testing.T) {
	s := NewBufferedSinkWith(10, 10*time.Millisecond).(*BufferedSink)
	// write a few entries
	for i := 0; i < 3; i++ {
		_ = s.Write(newLogEntry(INFO, "x", map[string]interface{}{}))
	}
	// close should not deadlock and flusher goroutine should stop
	done := make(chan struct{})
	go func() {
		_ = s.Close()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("BufferedSink.Close() timed out; flusher may not stop")
	}
}

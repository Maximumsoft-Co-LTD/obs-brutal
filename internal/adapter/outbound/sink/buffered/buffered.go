package buffered

import (
	stdout "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/sink/stdout"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"sync"
	"time"
)

type BufferedSink struct {
	port.SinkBase
	buffer     []*domain.LogEntry
	maxSize    int
	timeout    time.Duration
	lastFlush  time.Time
	mu         sync.Mutex
	stopCh     chan struct{}
	closedOnce sync.Once
	wg         sync.WaitGroup
	inner      port.Sink
}

func NewBufferedSink() port.Sink { return NewBufferedSinkWith(1000, 100*time.Millisecond) }
func NewBufferedSinkWith(size int, timeout time.Duration) port.Sink {
	s := &BufferedSink{buffer: make([]*domain.LogEntry, 0, size), maxSize: size, timeout: timeout, inner: stdout.NewFastStdoutSink()}
	s.startFlusher()
	return s
}

// NewBuf creates a buffered wrapper over the provided inner sink.
func NewBuf(inner port.Sink, size int, timeout time.Duration) port.Sink {
	if inner == nil {
		inner = stdout.NewFastStdoutSink()
	}
	s := &BufferedSink{buffer: make([]*domain.LogEntry, 0, size), maxSize: size, timeout: timeout, inner: inner}
	s.startFlusher()
	return s
}
func (s *BufferedSink) Name() string { return "buffer" }
func (s *BufferedSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffer = append(s.buffer, entry)
	if len(s.buffer) >= s.maxSize || (!s.lastFlush.IsZero() && time.Since(s.lastFlush) >= s.timeout) {
		return s.flushLocked()
	}
	return nil
}
func (s *BufferedSink) Close() error {
	var err error
	s.closedOnce.Do(func() {
		s.mu.Lock()
		if s.stopCh != nil {
			close(s.stopCh)
		}
		s.mu.Unlock()
		s.wg.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		err = s.flushLocked()
		if s.inner != nil {
			if cerr := s.inner.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	})
	return err
}

// retainCap bounds how many failed entries flushLocked keeps for retry
// during an inner-sink outage, so a prolonged outage cannot grow the
// buffer without limit.
func (s *BufferedSink) retainCap() int {
	if s.maxSize > 0 {
		return s.maxSize * 10
	}
	return 10000
}

func (s *BufferedSink) flushLocked() error {
	s.lastFlush = time.Now()
	if s.inner == nil {
		s.buffer = s.buffer[:0]
		return nil
	}
	// Attempt every entry. Entries whose write fails are kept for the
	// next flush instead of being discarded (silent loss) — bounded by
	// retainCap so an outage can't grow memory without limit.
	var firstErr error
	var remaining []*domain.LogEntry
	for _, e := range s.buffer {
		if err := s.inner.Write(e); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			remaining = append(remaining, e)
		}
	}
	if limit := s.retainCap(); len(remaining) > limit {
		remaining = remaining[len(remaining)-limit:]
	}
	s.buffer = append(s.buffer[:0], remaining...)
	return firstErr
}
func (s *BufferedSink) startFlusher() {
	s.mu.Lock()
	if s.stopCh != nil {
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	interval := s.timeout
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-t.C:
				s.mu.Lock()
				if len(s.buffer) > 0 {
					_ = s.flushLocked()
				}
				s.mu.Unlock()
			}
		}
	}()
}

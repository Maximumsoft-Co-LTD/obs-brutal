package buffered

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"obs-brutal/internal/util"
	"os"
	"sync"
	"time"
)

type BufferedSink struct {
	port.SinkBase
	buffer    []*domain.LogEntry
	maxSize   int
	timeout   time.Duration
	lastFlush time.Time
	mu        sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewBufferedSink() port.Sink { return NewBufferedSinkWith(1000, 100*time.Millisecond) }
func NewBufferedSinkWith(size int, timeout time.Duration) port.Sink {
	s := &BufferedSink{buffer: make([]*domain.LogEntry, 0, size), maxSize: size, timeout: timeout}
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
	s.mu.Lock()
	if s.stopCh != nil {
		close(s.stopCh)
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}
func (s *BufferedSink) flushLocked() error {
	for _, e := range s.buffer {
		_ = util.WriteJSONToWriter(os.Stdout, e)
	}
	s.buffer = s.buffer[:0]
	s.lastFlush = time.Now()
	return nil
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

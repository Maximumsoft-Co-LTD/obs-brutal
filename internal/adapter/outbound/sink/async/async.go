package async

import (
	"sync"
	"time"

	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
)

// Sink implements port.Sink with internal channel and worker pool
// to decouple caller goroutines from slow sinks

type Sink struct {
	port.SinkBase
	inner      port.Sink
	queue      chan *domain.LogEntry
	workers    int
	wg         sync.WaitGroup
	closedOnce sync.Once
	stopCh     chan struct{}
}

func New(inner port.Sink, queueSize, workers int) *Sink {
	if queueSize <= 0 {
		queueSize = 1000
	}
	if workers <= 0 {
		workers = 4
	}
	s := &Sink{inner: inner, queue: make(chan *domain.LogEntry, queueSize), workers: workers, stopCh: make(chan struct{})}
	s.start()
	return s
}

func (s *Sink) Name() string { return "async_sink" }

func (s *Sink) Write(entry *domain.LogEntry) error {
	select {
	case s.queue <- entry:
		return nil
	default:
		// drop on full to preserve caller latency
		return nil
	}
}

func (s *Sink) Close() error {
	s.closedOnce.Do(func() {
		close(s.stopCh)
		close(s.queue)
	})
	s.wg.Wait()
	return nil
}

func (s *Sink) start() {
	s.wg.Add(s.workers)
	for i := 0; i < s.workers; i++ {
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-s.stopCh:
					return
				case e, ok := <-s.queue:
					if !ok {
						return
					}
					if e != nil && s.inner != nil {
						_ = s.inner.Write(e)
					}
				}
			}
		}()
	}
}

// WithTimeout wraps an inner sink with periodic flush via Close()
// it closes automatically after duration d (best-effort)
func WithTimeout(inner port.Sink, d time.Duration) port.Sink {
	as := New(inner, 1000, 4)
	go func() {
		t := time.NewTimer(d)
		<-t.C
		_ = as.Close()
	}()
	return as
}

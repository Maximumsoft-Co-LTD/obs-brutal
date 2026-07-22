package async

import (
	"sync"
	"sync/atomic"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
)

// Sink implements port.Sink with an internal channel and worker pool to
// decouple caller goroutines from slow sinks.

type Sink struct {
	port.SinkBase
	inner      port.Sink
	queue      chan *domain.LogEntry
	workers    int
	wg         sync.WaitGroup
	closedOnce sync.Once
	closed     atomic.Bool
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
	// After Close the workers are gone and the queue must not be written
	// to. Drop silently — dropping is already this sink's contract when
	// the queue is full, and the alternative (send on a closed channel)
	// panics the process during shutdown.
	if s.closed.Load() {
		return nil
	}
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
		// Signal "no more writes", then let workers drain what is already
		// queued before they exit (drain, not drop). The queue is never
		// closed, so a Write racing Close cannot panic.
		s.closed.Store(true)
		close(s.stopCh)
		s.wg.Wait()
		if s.inner != nil {
			_ = s.inner.Close()
		}
	})
	return nil
}

func (s *Sink) start() {
	s.wg.Add(s.workers)
	for range s.workers {
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-s.stopCh:
					// Drain everything still buffered, then exit. Without
					// this, entries queued right before Close were dropped.
					for {
						select {
						case e := <-s.queue:
							s.writeInner(e)
						default:
							return
						}
					}
				case e := <-s.queue:
					s.writeInner(e)
				}
			}
		}()
	}
}

func (s *Sink) writeInner(e *domain.LogEntry) {
	if e != nil && s.inner != nil {
		_ = s.inner.Write(e)
	}
}

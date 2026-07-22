package base

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
)

// AsyncPipeline provides non-blocking async logging
type AsyncPipeline struct {
	logChan   chan *domain.LogEntry
	batchChan chan []*domain.LogEntry

	processed atomic.Uint64
	dropped   atomic.Uint64
	batches   atomic.Uint64
	errors    atomic.Uint64

	batchSize    int
	flushTimeout time.Duration
	workerCount  int

	ctx    context.Context
	cancel context.CancelFunc

	// Shutdown is two-phase so Stop() can drain instead of drop: quit
	// tells the batch worker to drain logChan and flush, then batchChan
	// is closed so sink workers finish everything already batched.
	quit     chan struct{}
	stopped  atomic.Bool
	stopOnce sync.Once
	batchWg  sync.WaitGroup
	sinkWg   sync.WaitGroup

	sinks []port.Sink

	batchPool sync.Pool

	metricsHook func(AsyncStats)

	// Backpressure controls behavior when log queue is full
	policy   atomic.Int32
	blockFor atomic.Int64
}

// AsyncBackpressurePolicy defines how the async pipeline reacts
// when its internal queue is full.
type AsyncBackpressurePolicy int

const (
	// DropLatest drops the incoming log when queue is full (default).
	DropLatest AsyncBackpressurePolicy = iota
	// DropOldest removes the oldest pending log to make room for the new one.
	DropOldest
	// BlockShort blocks up to blockFor duration to enqueue; drops if still full.
	BlockShort
)

func NewAsyncPipeline(batchSize, workerCount int, flushTimeout time.Duration, sinks ...port.Sink) *AsyncPipeline {
	ctx, cancel := context.WithCancel(context.Background())
	ap := &AsyncPipeline{
		logChan:      make(chan *domain.LogEntry, batchSize*4),
		batchChan:    make(chan []*domain.LogEntry, workerCount*2),
		batchSize:    batchSize,
		flushTimeout: flushTimeout,
		workerCount:  workerCount,
		ctx:          ctx,
		cancel:       cancel,
		quit:         make(chan struct{}),
		sinks:        sinks,
		batchPool:    sync.Pool{New: func() interface{} { return make([]*domain.LogEntry, 0, batchSize) }},
	}
	ap.policy.Store(int32(DropLatest))
	ap.start()
	return ap
}

// WriteAsync enqueues a log entry according to the configured
// backpressure policy. Returns true if accepted, false if dropped.
func (ap *AsyncPipeline) WriteAsync(entry *domain.LogEntry) bool {
	// After Stop the workers are gone; report rejection so the caller
	// falls back to WriteSync and the entry still reaches the sinks.
	if ap.stopped.Load() {
		return false
	}
	switch AsyncBackpressurePolicy(ap.policy.Load()) {
	case DropOldest:
		// Try fast path enqueue
		select {
		case ap.logChan <- entry:
			return true
		default:
			// Queue appears full; drop one oldest if possible
			droppedOldest := false
			select {
			case <-ap.logChan:
				droppedOldest = true
			default:
				// couldn't drop (race or not yet full)
			}
			if droppedOldest {
				ap.dropped.Add(1)
				// attempt enqueue after making room
				select {
				case ap.logChan <- entry:
					return true
				default:
					// rare contention: fall through to drop incoming
				}
			}
			// drop the incoming entry
			ap.dropped.Add(1)
			return false
		}
	case BlockShort:
		blockFor := time.Duration(ap.blockFor.Load())
		if blockFor <= 0 {
			blockFor = 10 * time.Millisecond
		}
		timer := time.NewTimer(blockFor)
		defer timer.Stop()
		select {
		case ap.logChan <- entry:
			return true
		case <-timer.C:
			ap.dropped.Add(1)
			return false
		}
	case DropLatest:
		fallthrough
	default:
		select {
		case ap.logChan <- entry:
			return true
		default:
			ap.dropped.Add(1)
			return false
		}
	}
}

func (ap *AsyncPipeline) start() {
	ap.batchWg.Add(1)
	go ap.batchWorker()
	for i := 0; i < ap.workerCount; i++ {
		ap.sinkWg.Add(1)
		go ap.sinkWorker()
	}
}
func (ap *AsyncPipeline) batchWorker() {
	defer ap.batchWg.Done()
	batch := ap.batchPool.Get().([]*domain.LogEntry)
	defer ap.batchPool.Put(batch)
	ticker := time.NewTicker(ap.flushTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-ap.quit:
			// Drain whatever producers already enqueued, then flush the
			// final batch. Dropping here is what used to lose every log
			// a short-lived process wrote right before Close.
			for {
				select {
				case entry := <-ap.logChan:
					batch = append(batch, entry)
					if len(batch) >= ap.batchSize {
						ap.flushBatch(batch)
						clear(batch)
						batch = batch[:0]
					}
				default:
					ap.flushBatch(batch)
					return
				}
			}
		case entry := <-ap.logChan:
			batch = append(batch, entry)
			if len(batch) >= ap.batchSize {
				ap.flushBatch(batch)
				clear(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				ap.flushBatch(batch)
				clear(batch)
				batch = batch[:0]
			}
		}
	}
}

func (ap *AsyncPipeline) flushBatch(batch []*domain.LogEntry) {
	if len(batch) == 0 {
		return
	}
	// With no sink workers there is no consumer for batchChan — write
	// inline instead of blocking forever on the send below.
	if ap.workerCount <= 0 {
		for _, e := range batch {
			ap.WriteSync(e)
			ap.processed.Add(1)
		}
		ap.batches.Add(1)
		if ap.metricsHook != nil {
			ap.metricsHook(ap.Stats())
		}
		return
	}
	copyBatch := make([]*domain.LogEntry, len(batch))
	copy(copyBatch, batch)
	// Blocking send is safe: sink workers only exit after batchChan is
	// closed by Stop, which happens strictly after this worker returns.
	ap.batchChan <- copyBatch
	ap.batches.Add(1)
	if ap.metricsHook != nil {
		ap.metricsHook(ap.Stats())
	}
}

func (ap *AsyncPipeline) sinkWorker() {
	defer ap.sinkWg.Done()
	for batch := range ap.batchChan {
		for _, e := range batch {
			for _, s := range ap.sinks {
				if s != nil {
					if err := s.Write(e); err != nil {
						ap.errors.Add(1)
					}
				}
			}
			ap.processed.Add(1)
		}
		if ap.metricsHook != nil {
			ap.metricsHook(ap.Stats())
		}
	}
}

// Stop drains the pipeline before shutting it down: pending entries in
// logChan are batched and flushed, in-flight batches are written by the
// sink workers, and only then do the goroutines exit. Safe to call more
// than once. Writes issued after Stop are rejected by WriteAsync so
// callers fall back to their synchronous path.
func (ap *AsyncPipeline) Stop() {
	ap.stopOnce.Do(func() {
		ap.stopped.Store(true)
		close(ap.quit)
		ap.batchWg.Wait()
		close(ap.batchChan)
		ap.sinkWg.Wait()
		ap.cancel()
	})
}

type AsyncStats struct{ Processed, Dropped, Batches, Errors, QueueSize uint64 }

func (ap *AsyncPipeline) Stats() AsyncStats {
	return AsyncStats{
		Processed: ap.processed.Load(),
		Dropped:   ap.dropped.Load(),
		Batches:   ap.batches.Load(),
		Errors:    ap.errors.Load(),
		QueueSize: uint64(len(ap.logChan)),
	}
}
func (ap *AsyncPipeline) WriteSync(entry *domain.LogEntry) {
	for _, s := range ap.sinks {
		if s != nil {
			if err := s.Write(entry); err != nil {
				ap.errors.Add(1)
			}
		}
	}
}

// SetMetricsHook installs a callback to receive periodic AsyncStats updates
// SetMetricsHook registers a callback invoked after batches are flushed
// and after sinks consume a batch. Useful for exporting AsyncStats.
func (ap *AsyncPipeline) SetMetricsHook(h func(AsyncStats)) { ap.metricsHook = h }

// AsyncLogBrt wraps UnifiedLogBrt with async pipeline
type AsyncLogBrt struct {
	*UnifiedLogBrt
	pipeline *AsyncPipeline
}

func NewAsyncLogBrt(level domain.Level, sinks ...port.Sink) *AsyncLogBrt {
	pipeline := NewAsyncPipeline(1000, 4, 100*time.Millisecond, sinks...)
	// Pass sinks to the embedded UnifiedLogBrt too so chained calls
	// (e.g. log.F("k", v).Info("msg")) land on the same sinks as direct
	// calls. Without this, .clone() returns a UnifiedLogBrt with the
	// default stdout sink and bypasses any caller-supplied sinks.
	base := NewUnifiedLogBrt(level, sinks...)
	return &AsyncLogBrt{UnifiedLogBrt: base, pipeline: pipeline}
}

// NewAsyncLogBrtCfg constructs an async logger with custom pipeline settings.
func NewAsyncLogBrtCfg(batchSize, workerCount int, flushTimeout time.Duration, level domain.Level, sinks ...port.Sink) *AsyncLogBrt {
	pipeline := NewAsyncPipeline(batchSize, workerCount, flushTimeout, sinks...)
	base := NewUnifiedLogBrt(level, sinks...)
	return &AsyncLogBrt{UnifiedLogBrt: base, pipeline: pipeline}
}

func (al *AsyncLogBrt) log(level domain.Level, msg string) {
	if level < al.Level() {
		return
	}
	fields := al.FieldsCopy()
	var entry *domain.LogEntry
	if len(fields) == 0 {
		entry = &domain.LogEntry{Level: level, Msg: msg, Timestamp: time.Now()}
	} else {
		entry = &domain.LogEntry{Level: level, Msg: msg, Timestamp: time.Now(), F: fields}
		Promote(entry)
	}
	if !al.pipeline.WriteAsync(entry) {
		al.pipeline.WriteSync(entry)
	}
	al.IncCount()
}
func (al *AsyncLogBrt) Debug(msg string) { al.log(domain.DebugLevel, msg) }
func (al *AsyncLogBrt) Info(msg string)  { al.log(domain.InfoLevel, msg) }
func (al *AsyncLogBrt) Warn(msg string)  { al.log(domain.WarnLevel, msg) }
func (al *AsyncLogBrt) Error(msg string) { al.log(domain.ErrorLevel, msg) }
func (al *AsyncLogBrt) Fatal(msg string) { al.log(domain.FatalLevel, msg) }
func (al *AsyncLogBrt) Debugf(format string, args ...interface{}) {
	al.log(domain.DebugLevel, fmt.Sprintf(format, args...))
}
func (al *AsyncLogBrt) Infof(format string, args ...interface{}) {
	al.log(domain.InfoLevel, fmt.Sprintf(format, args...))
}
func (al *AsyncLogBrt) Warnf(format string, args ...interface{}) {
	al.log(domain.WarnLevel, fmt.Sprintf(format, args...))
}
func (al *AsyncLogBrt) Errorf(format string, args ...interface{}) {
	al.log(domain.ErrorLevel, fmt.Sprintf(format, args...))
}
func (al *AsyncLogBrt) Fatalf(format string, args ...interface{}) {
	al.log(domain.FatalLevel, fmt.Sprintf(format, args...))
}
func (al *AsyncLogBrt) GetAsyncStats() AsyncStats { return al.pipeline.Stats() }
func (al *AsyncLogBrt) Stop()                     { al.pipeline.Stop() }

// SetMetricsHook registers a metrics callback to the underlying async pipeline
func (al *AsyncLogBrt) SetMetricsHook(h func(AsyncStats)) {
	if al != nil && al.pipeline != nil {
		al.pipeline.SetMetricsHook(h)
	}
}

// SetBackpressurePolicy configures behavior when the queue is full
// SetBackpressurePolicy sets the queue-full behavior for the underlying
// async pipeline. blockFor is only used with BlockShort.
func (al *AsyncLogBrt) SetBackpressurePolicy(policy AsyncBackpressurePolicy, blockFor time.Duration) {
	if al != nil && al.pipeline != nil {
		al.pipeline.policy.Store(int32(policy))
		al.pipeline.blockFor.Store(blockFor.Nanoseconds())
	}
}

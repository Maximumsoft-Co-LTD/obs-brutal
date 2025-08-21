// Package core provides async logging pipeline for ultra-high performance
package core

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"obs-brutal/internal/core/domain"
)

// AsyncPipeline provides non-blocking async logging with Go 1.25 optimizations
type AsyncPipeline struct {
	// Go 1.25: Enhanced channel performance
	logChan   chan *domain.LogEntry
	batchChan chan []*domain.LogEntry

	// Atomic counters (Go 1.25 optimized)
	processed atomic.Uint64
	dropped   atomic.Uint64
	batches   atomic.Uint64

	// Configuration
	batchSize    int
	flushTimeout time.Duration
	workerCount  int

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Sinks
	sinks []Sink

	// Go 1.25: Pre-allocated slices with better memory management
	batchPool sync.Pool
}

// NewAsyncPipeline creates high-performance async logging pipeline
func NewAsyncPipeline(batchSize, workerCount int, flushTimeout time.Duration, sinks ...Sink) *AsyncPipeline {
	ctx, cancel := context.WithCancel(context.Background())

	ap := &AsyncPipeline{
		logChan:      make(chan *domain.LogEntry, batchSize*4), // Large buffer
		batchChan:    make(chan []*domain.LogEntry, workerCount*2),
		batchSize:    batchSize,
		flushTimeout: flushTimeout,
		workerCount:  workerCount,
		ctx:          ctx,
		cancel:       cancel,
		sinks:        sinks,
		batchPool: sync.Pool{
			New: func() interface{} {
				// Go 1.25: Pre-allocate with optimal capacity
				return make([]*domain.LogEntry, 0, batchSize)
			},
		},
	}

	// Start async workers
	ap.start()

	return ap
}

// WriteAsync writes log entry asynchronously (non-blocking)
func (ap *AsyncPipeline) WriteAsync(entry *domain.LogEntry) bool {
	select {
	case ap.logChan <- entry:
		return true
	default:
		// Channel full - increment dropped counter
		ap.dropped.Add(1)
		return false
	}
}

// start initializes async workers
func (ap *AsyncPipeline) start() {
	// Start batcher worker
	ap.wg.Add(1)
	go ap.batchWorker()

	// Start sink workers
	for i := 0; i < ap.workerCount; i++ {
		ap.wg.Add(1)
		go ap.sinkWorker()
	}
}

// batchWorker collects entries into batches
func (ap *AsyncPipeline) batchWorker() {
	defer ap.wg.Done()

	// Go 1.25: Use pool for batch allocation
	batch := ap.batchPool.Get().([]*domain.LogEntry)
	defer ap.batchPool.Put(batch)

	ticker := time.NewTicker(ap.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ap.ctx.Done():
			// Flush remaining entries
			if len(batch) > 0 {
				ap.flushBatch(batch)
			}
			return

		case entry := <-ap.logChan:
			batch = append(batch, entry)

			// Flush when batch is full
			if len(batch) >= ap.batchSize {
				ap.flushBatch(batch)
				// Go 1.25: Use clear() for efficient reset
				clear(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Flush on timeout
			if len(batch) > 0 {
				ap.flushBatch(batch)
				clear(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch sends batch to sink workers
func (ap *AsyncPipeline) flushBatch(batch []*domain.LogEntry) {
	if len(batch) == 0 {
		return
	}

	// Create batch copy for workers
	batchCopy := make([]*domain.LogEntry, len(batch))
	copy(batchCopy, batch)

	select {
	case ap.batchChan <- batchCopy:
		ap.batches.Add(1)
	case <-ap.ctx.Done():
		return
	}
}

// sinkWorker processes batches and writes to sinks
func (ap *AsyncPipeline) sinkWorker() {
	defer ap.wg.Done()

	for {
		select {
		case <-ap.ctx.Done():
			return

		case batch := <-ap.batchChan:
			// Process batch
			for _, entry := range batch {
				// Write to all sinks concurrently
				for _, sink := range ap.sinks {
					if sink != nil {
						sink.Write(entry)
					}
				}
				ap.processed.Add(1)
			}
		}
	}
}

// Stop gracefully shuts down the async pipeline
func (ap *AsyncPipeline) Stop() {
	ap.cancel()
	ap.wg.Wait()
}

// Stats returns pipeline statistics
func (ap *AsyncPipeline) Stats() AsyncStats {
	return AsyncStats{
		Processed: ap.processed.Load(),
		Dropped:   ap.dropped.Load(),
		Batches:   ap.batches.Load(),
		QueueSize: uint64(len(ap.logChan)),
	}
}

// AsyncStats holds pipeline statistics
type AsyncStats struct {
	Processed uint64 `json:"processed"`
	Dropped   uint64 `json:"dropped"`
	Batches   uint64 `json:"batches"`
	QueueSize uint64 `json:"queue_size"`
}

// AsyncLogger wraps UnifiedLogger with async pipeline
type AsyncLogger struct {
	*UnifiedLogger
	pipeline *AsyncPipeline
}

// NewAsyncLogger creates logger with async pipeline
func NewAsyncLogger(level Level, sinks ...Sink) *AsyncLogger {
	pipeline := NewAsyncPipeline(1000, 4, 100*time.Millisecond, sinks...)

	// Create base logger without sinks (pipeline handles them)
	baseLogger := NewUnifiedLogger(level)

	return &AsyncLogger{
		UnifiedLogger: baseLogger,
		pipeline:      pipeline,
	}
}

// Override log method to use async pipeline
func (al *AsyncLogger) log(level domain.Level, msg string) {
	// Fast level check
	if level < al.level {
		return
	}

	// Create log entry efficiently
	entry := &domain.LogEntry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}, len(al.fields)),
	}

	// Copy fields efficiently
	for k, v := range al.fields {
		entry.Fields[k] = v
	}

	// Write async (non-blocking)
	if !al.pipeline.WriteAsync(entry) {
		// Fallback to sync if pipeline is full
		for _, sink := range al.pipeline.sinks {
			if sink != nil {
				sink.Write(entry)
			}
		}
	}

	// Update counter
	if al.logCount != nil {
		al.logCount.Add(1)
	}
}

// GetAsyncStats returns async pipeline statistics
func (al *AsyncLogger) GetAsyncStats() AsyncStats {
	return al.pipeline.Stats()
}

// Stop gracefully stops async pipeline
func (al *AsyncLogger) Stop() {
	al.pipeline.Stop()
}

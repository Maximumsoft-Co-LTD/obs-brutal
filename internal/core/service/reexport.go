package service

import (
	"context"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	b "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/service/base"
	strat "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/service/strategy"
)

// ===== Base Logger Re-exports =====

// LogBrt defines the fluent logging interface (re-export from base).
type LogBrt = b.LogBrt

// UnifiedLogBrt is the synchronous logger implementation (re-export).
type UnifiedLogBrt = b.UnifiedLogBrt

// AsyncLogBrt is the asynchronous logger implementation (re-export).
type AsyncLogBrt = b.AsyncLogBrt

// AsyncPipeline is the underlying async pipeline type (re-export).
type AsyncPipeline = b.AsyncPipeline

// NewUnifiedLogBrt constructs a synchronous logger.
func NewUnifiedLogBrt(level domain.Level, sinks ...port.Sink) *UnifiedLogBrt {
	return b.NewUnifiedLogBrt(level, sinks...)
}

// NewAsyncLogBrt constructs an asynchronous logger with default pipeline settings.
func NewAsyncLogBrt(level domain.Level, sinks ...port.Sink) *AsyncLogBrt {
	return b.NewAsyncLogBrt(level, sinks...)
}

// NewAsyncLogBrtCfg constructs an async logger with custom pipeline settings.
func NewAsyncLogBrtCfg(batchSize, workerCount int, flushTimeout time.Duration, level domain.Level, sinks ...port.Sink) *AsyncLogBrt {
	return b.NewAsyncLogBrtCfg(batchSize, workerCount, flushTimeout, level, sinks...)
}

// NewAsyncPipeline constructs an async pipeline with custom settings.
func NewAsyncPipeline(batchSize, workerCount int, flushTimeout time.Duration, sinks ...port.Sink) *AsyncPipeline {
	return b.NewAsyncPipeline(batchSize, workerCount, flushTimeout, sinks...)
}

// NewAsyncLogBrtWithTelemetry builds an async logger and wires its stats into the TelemetryProvider
func NewAsyncLogBrtWithTelemetry(tp port.TelemetryProvider, level domain.Level, sinks ...port.Sink) *AsyncLogBrt {
	al := b.NewAsyncLogBrt(level, sinks...)
	if tp == nil || tp.Meter() == nil {
		return al
	}
	meter := tp.Meter()
	labels := map[string]string{"component": "logger"}
	var prev b.AsyncStats
	var hookMu sync.Mutex
	al.SetMetricsHook(func(s b.AsyncStats) {
		hookMu.Lock()
		defer hookMu.Unlock()
		incN := func(name string, n uint64) {
			if n == 0 {
				return
			}
			// Avoid tight loops for very large deltas
			if n > 1000 {
				meter.ObserveHistogram(context.Background(), name+"_delta", float64(n), labels)
				return
			}
			for i := uint64(0); i < n; i++ {
				meter.IncCounter(context.Background(), name, labels)
			}
		}
		// compute deltas
		if s.Processed >= prev.Processed {
			incN("obs_async_processed_total", s.Processed-prev.Processed)
		}
		if s.Dropped >= prev.Dropped {
			incN("obs_async_dropped_total", s.Dropped-prev.Dropped)
		}
		if s.Batches >= prev.Batches {
			incN("obs_async_batches_total", s.Batches-prev.Batches)
		}
		if s.Errors >= prev.Errors {
			incN("obs_async_errors_total", s.Errors-prev.Errors)
		}
		prev = s
		// queue size as gauge-like via histogram
		meter.ObserveHistogram(context.Background(), "obs_async_queue_size", float64(s.QueueSize), labels)
	})
	return al
}

// ===== Strategy Re-exports =====

// StrategyLogBrt is a logger with pluggable filter/sampler/masker strategies.
type StrategyLogBrt = strat.StrategyLogBrt

// StrategyManager manages runtime strategies; exposed for advanced usage.
type StrategyManager = strat.Manager

// NewStrategyLogBrt constructs a strategy-enabled logger.
func NewStrategyLogBrt(level domain.Level, sinks ...port.Sink) *StrategyLogBrt {
	return strat.NewStrategyLogBrt(level, sinks...)
}

// NewStrategyManager constructs an empty strategy manager.
func NewStrategyManager() *StrategyManager { return strat.NewManager() }

// FilterStrategy is the interface for log filters.
type FilterStrategy = port.FilterStrategy

// SamplerStrategy is the interface for log samplers.
type SamplerStrategy = port.SamplerStrategy

// MaskingStrategy is the interface for field maskers.
type MaskingStrategy = port.MaskingStrategy

// NewLevelFilter creates a level-based filter (inclusive range).
func NewLevelFilter(min, max domain.Level) FilterStrategy { return strat.NewLevelFilter(min, max) }

// NewRateSampler creates a rate-based sampler (0..1).
func NewRateSampler(rate float64) SamplerStrategy { return strat.NewRateSampler(rate) }

// NewAdaptiveSampler creates a simple adaptive sampler.
func NewAdaptiveSampler(baseRate, minRate, maxRate float64) SamplerStrategy {
	return strat.NewAdaptiveSampler(baseRate, minRate, maxRate)
}

// ===== Async Backpressure Policies =====

// AsyncBackpressurePolicy re-exports base policy type for facade users.
type AsyncBackpressurePolicy = b.AsyncBackpressurePolicy

// Backpressure policy constants for queue-full behavior.
const (
	DropLatest = b.DropLatest
	DropOldest = b.DropOldest
	BlockShort = b.BlockShort
)

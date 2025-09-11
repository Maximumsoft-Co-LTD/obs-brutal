package service

import (
	"context"
	"time"

	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	b "obs-brutal/internal/core/service/base"
	strat "obs-brutal/internal/core/service/strategy"
)

// OTelLogBrt composes StrategyLogBrt and adds OTEL metrics/tracing hooks.
type OTelLogBrt struct {
	Strategy *strat.StrategyLogBrt
	tp       port.TelemetryProvider
}

// NewOTelLogBrtWithProvider creates an OTEL-enabled logger using a
// TelemetryProvider supplied by the caller (adapter/outbound).
func NewOTelLogBrtWithProvider(tp port.TelemetryProvider, level domain.Level, sinks ...port.Sink) *OTelLogBrt {
	stratLogger := strat.NewStrategyLogBrt(level, sinks...)
	ol := &OTelLogBrt{Strategy: stratLogger, tp: tp}
	ol.installPipelineMetricsHook()
	return ol
}

// Note: construction of concrete telemetry providers lives in adapter/outbound.

func (ol *OTelLogBrt) record(level domain.Level, start time.Time) {
	d := time.Since(start)
	// prefer context from base fields if present
	metCtx := context.Background()
	baseFields := ol.Strategy.Base.FieldsCopy()
	if c, ok := baseFields["context"].(context.Context); ok {
		metCtx = c
	}
	if ol.tp != nil {
		labels := map[string]string{}
		if svcName, ok := baseFields["service"].(string); ok && svcName != "" {
			labels["service"] = svcName
		}
		ol.tp.RecordLog(metCtx, level, d, labels)
	}
}

// Debug logs a message at debug level and records an OTEL metric.
func (ol *OTelLogBrt) Debug(msg string) {
	start := time.Now()
	ol.Strategy.Debug(msg)
	ol.record(domain.DebugLevel, start)
}

// Info logs a message at info level and records an OTEL metric.
func (ol *OTelLogBrt) Info(msg string) {
	start := time.Now()
	ol.Strategy.Info(msg)
	ol.record(domain.InfoLevel, start)
}
func (ol *OTelLogBrt) Warn(msg string) {
	start := time.Now()
	ol.Strategy.Warn(msg)
	ol.record(domain.WarnLevel, start)
}
func (ol *OTelLogBrt) Error(msg string) {
	start := time.Now()
	ol.Strategy.Error(msg)
	ol.record(domain.ErrorLevel, start)
}
func (ol *OTelLogBrt) Fatal(msg string) {
	start := time.Now()
	ol.Strategy.Fatal(msg)
	ol.record(domain.FatalLevel, start)
}
func (ol *OTelLogBrt) Debugf(f string, a ...interface{}) {
	start := time.Now()
	ol.Strategy.Debugf(f, a...)
	ol.record(domain.DebugLevel, start)
}
func (ol *OTelLogBrt) Infof(f string, a ...interface{}) {
	start := time.Now()
	ol.Strategy.Infof(f, a...)
	ol.record(domain.InfoLevel, start)
}
func (ol *OTelLogBrt) Warnf(f string, a ...interface{}) {
	start := time.Now()
	ol.Strategy.Warnf(f, a...)
	ol.record(domain.WarnLevel, start)
}
func (ol *OTelLogBrt) Errorf(f string, a ...interface{}) {
	start := time.Now()
	ol.Strategy.Errorf(f, a...)
	ol.record(domain.ErrorLevel, start)
}
func (ol *OTelLogBrt) Fatalf(f string, a ...interface{}) {
	start := time.Now()
	ol.Strategy.Fatalf(f, a...)
	ol.record(domain.FatalLevel, start)
}

// Fluent methods delegate and return the same wrapper (which implements base.LogBrt)
func (ol *OTelLogBrt) F(k string, v interface{}) LogBrt {
	ol.Strategy = ol.Strategy.F(k, v).(*strat.StrategyLogBrt)
	return ol
}
func (ol *OTelLogBrt) Fs(m map[string]interface{}) LogBrt {
	ol.Strategy = ol.Strategy.Fs(m).(*strat.StrategyLogBrt)
	return ol
}

// Ctx attaches a context for correlation/propagation.
func (ol *OTelLogBrt) Ctx(ctx context.Context) LogBrt {
	ol.Strategy = ol.Strategy.Ctx(ctx).(*strat.StrategyLogBrt)
	return ol
}
func (ol *OTelLogBrt) TraceID(id string) LogBrt   { return ol.F("trace_id", id) }
func (ol *OTelLogBrt) UserID(id string) LogBrt    { return ol.F("user_id", id) }
func (ol *OTelLogBrt) RequestID(id string) LogBrt { return ol.F("request_id", id) }
func (ol *OTelLogBrt) WithError(err error) LogBrt {
	if err == nil {
		return ol
	}
	return ol.F("error", err.Error())
}

// LogCount forwards to underlying base logger
func (ol *OTelLogBrt) LogCount() int64 { return ol.Strategy.Base.LogCount() }

// Helpers for security integration
func (ol *OTelLogBrt) FieldsCopy() map[string]interface{} { return ol.Strategy.Base.FieldsCopy() }
func (ol *OTelLogBrt) WithOnlyFields(m map[string]interface{}) LogBrt {
	ol.Strategy.Base = ol.Strategy.Base.WithOnlyFields(m)
	return ol
}
func (ol *OTelLogBrt) Context() context.Context {
	if c, ok := ol.Strategy.Base.FieldsCopy()["context"].(context.Context); ok {
		return c
	}
	return nil
}

// GetTelemetryProvider returns the underlying TelemetryProvider (may be nil).
func (ol *OTelLogBrt) GetTelemetryProvider() port.TelemetryProvider { return ol.tp }

// Strategy management passthroughs
// AddFilter appends a filter strategy.
func (ol *OTelLogBrt) AddFilter(f port.FilterStrategy) {
	if ol.Strategy != nil {
		ol.Strategy.AddFilter(f)
	}
}

// AddSampler appends a sampler strategy.
func (ol *OTelLogBrt) AddSampler(s port.SamplerStrategy) {
	if ol.Strategy != nil {
		ol.Strategy.AddSampler(s)
	}
}

// AddMasker appends a masker strategy.
func (ol *OTelLogBrt) AddMasker(m port.MaskingStrategy) {
	if ol.Strategy != nil {
		ol.Strategy.AddMasker(m)
	}
}

// RemoveStrategy removes a strategy by kind and name.
func (ol *OTelLogBrt) RemoveStrategy(t, n string) {
	if ol.Strategy != nil {
		ol.Strategy.RemoveStrategy(t, n)
	}
}

// WithSpan starts a span using the telemetry provider and returns
// the child context and the same logger bound to it.
func (ol *OTelLogBrt) WithSpan(ctx context.Context, name string) (context.Context, *OTelLogBrt) {
	if ol.tp == nil || ol.tp.Tracer() == nil {
		return ctx, ol
	}
	spanCtx, _ := ol.tp.Tracer().StartSpan(ctx, name)
	_ = ol.Ctx(spanCtx)
	return spanCtx, ol
}

// installPipelineMetricsHook wires async pipeline stats to the telemetry provider's meter
func (ol *OTelLogBrt) installPipelineMetricsHook() {
	if ol.tp == nil || ol.Strategy == nil {
		return
	}
	meter := ol.tp.Meter()
	if meter == nil {
		return
	}
	labels := map[string]string{"component": "logger"}
	if svc, ok := ol.Strategy.Base.FieldsCopy()["service"].(string); ok && svc != "" {
		labels["service"] = svc
	}
	var prev b.AsyncStats
	ol.Strategy.SetMetricsHook(func(s b.AsyncStats) {
		incN := func(name string, n uint64) {
			if n == 0 {
				return
			}
			if n > 1000 {
				meter.ObserveHistogram(context.Background(), name+"_delta", float64(n), labels)
				return
			}
			for i := uint64(0); i < n; i++ {
				meter.IncCounter(context.Background(), name, labels)
			}
		}
		if s.Processed >= prev.Processed {
			incN("obs_async_processed_total", s.Processed-prev.Processed)
		}
		if s.Dropped >= prev.Dropped {
			incN("obs_async_dropped_total", s.Dropped-prev.Dropped)
		}
		if s.Batches >= prev.Batches {
			incN("obs_async_batches_total", s.Batches-prev.Batches)
		}
		prev = s
		meter.ObserveHistogram(context.Background(), "obs_async_queue_size", float64(s.QueueSize), labels)
	})
}

package strategy

import (
	"context"
	"fmt"
	"time"

	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	b "obs-brutal/internal/core/service/base"
)

// StrategyLogBrt augments a base logger with strategies and async pipeline
type StrategyLogBrt struct {
	Base       *b.UnifiedLogBrt
	strategies *Manager
	async      *b.AsyncPipeline
}

func NewStrategyLogBrt(level domain.Level, sinks ...port.Sink) *StrategyLogBrt {
	strategies := NewManager()
	strategies.AddMasker(NewRegexMaskingStrategy())
	strategies.AddSampler(NewRateSampler(1.0))
	async := b.NewAsyncPipeline(1000, 4, 100*time.Millisecond, sinks...)
	base := b.NewUnifiedLogBrt(level)
	return &StrategyLogBrt{Base: base, strategies: strategies, async: async}
}

func (sl *StrategyLogBrt) log(level domain.Level, msg string) {
	if level < sl.Base.Level() {
		return
	}
	entry := &domain.LogEntry{Level: level, Msg: msg, Timestamp: time.Now(), F: sl.Base.FieldsCopy()}
	b.Promote(entry)
	processed := sl.strategies.ProcessEntry(entry)
	if processed == nil {
		return
	}
	if !sl.async.WriteAsync(processed) {
		sl.async.WriteSync(processed)
	}
	sl.Base.IncCount()
}
func (sl *StrategyLogBrt) Debug(msg string) { sl.log(domain.DebugLevel, msg) }
func (sl *StrategyLogBrt) Info(msg string)  { sl.log(domain.InfoLevel, msg) }
func (sl *StrategyLogBrt) Warn(msg string)  { sl.log(domain.WarnLevel, msg) }
func (sl *StrategyLogBrt) Error(msg string) { sl.log(domain.ErrorLevel, msg) }
func (sl *StrategyLogBrt) Fatal(msg string) { sl.log(domain.FatalLevel, msg) }
func (sl *StrategyLogBrt) Debugf(format string, args ...interface{}) {
	sl.log(domain.DebugLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Infof(format string, args ...interface{}) {
	sl.log(domain.InfoLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Warnf(format string, args ...interface{}) {
	sl.log(domain.WarnLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Errorf(format string, args ...interface{}) {
	sl.log(domain.ErrorLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Fatalf(format string, args ...interface{}) {
	sl.log(domain.FatalLevel, fmt.Sprintf(format, args...))
}

// Fluent chaining delegates to base
func (sl *StrategyLogBrt) F(key string, value interface{}) b.LogBrt {
	sl.Base = sl.Base.F(key, value).(*b.UnifiedLogBrt)
	return sl
}
func (sl *StrategyLogBrt) Fs(fields map[string]interface{}) b.LogBrt {
	if len(fields) == 0 {
		return sl
	}
	sl.Base = sl.Base.Fs(fields).(*b.UnifiedLogBrt)
	return sl
}
func (sl *StrategyLogBrt) Ctx(ctx context.Context) b.LogBrt {
	if ctx == nil {
		return sl
	}
	sl.Base = sl.Base.Ctx(ctx).(*b.UnifiedLogBrt)
	return sl
}
func (sl *StrategyLogBrt) TraceID(id string) b.LogBrt   { return sl.F("trace_id", id) }
func (sl *StrategyLogBrt) UserID(id string) b.LogBrt    { return sl.F("user_id", id) }
func (sl *StrategyLogBrt) RequestID(id string) b.LogBrt { return sl.F("request_id", id) }
func (sl *StrategyLogBrt) WithError(err error) b.LogBrt {
	if err == nil {
		return sl
	}
	return sl.F("error", err.Error())
}
func (sl *StrategyLogBrt) LogCount() int64 { return sl.Base.LogCount() }

// Strategy management
func (sl *StrategyLogBrt) AddFilter(f port.FilterStrategy)   { sl.strategies.AddFilter(f) }
func (sl *StrategyLogBrt) AddSampler(s port.SamplerStrategy) { sl.strategies.AddSampler(s) }
func (sl *StrategyLogBrt) AddMasker(m port.MaskingStrategy)  { sl.strategies.AddMasker(m) }
func (sl *StrategyLogBrt) RemoveStrategy(t, n string)        { sl.strategies.RemoveStrategy(t, n) }

// SetMetricsHook registers a metrics callback to the underlying async pipeline (if present)
func (sl *StrategyLogBrt) SetMetricsHook(h func(b.AsyncStats)) {
	if sl.async != nil {
		// adapt types since AsyncStats in base package
		sl.async.SetMetricsHook(func(s b.AsyncStats) { h(s) })
	}
}

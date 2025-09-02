package strategy

import (
    "obs-brutal/internal/core/domain"
    "obs-brutal/internal/core/port"
)

// memSink is a minimal sink for examples/tests
type memSink struct{ port.SinkBase }
func (m *memSink) Write(*domain.LogEntry) error { return nil }
func (m *memSink) Name() string                 { return "mem" }

// Example showing StrategyLogBrt with filter/sampler/masker.
func ExampleStrategyLogBrt_policies() {
    s := NewStrategyLogBrt(domain.InfoLevel, &memSink{})
    s.AddFilter(NewLevelFilter(domain.InfoLevel, domain.ErrorLevel))
    s.AddSampler(NewRateSampler(1.0))
    s.AddMasker(NewRegexMaskingStrategy())
    s.F("email", "john.doe@example.com").Info("masked")
}

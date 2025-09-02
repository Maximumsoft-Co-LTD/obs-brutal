// Package stdout contains stdout-related sink helpers.
package stdout

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
)

// ToggleSink wraps another sink and conditionally forwards writes
// when enabled is true.
type ToggleSink struct {
	port.SinkBase
	inner   port.Sink
	enabled bool
}

func NewToggleSink(inner port.Sink, enabled bool) port.Sink {
	return &ToggleSink{inner: inner, enabled: enabled}
}
// Name returns the wrapped sink name or toggle(nil) if inner is nil.
func (t *ToggleSink) Name() string {
	if t.inner == nil {
		return "toggle(nil)"
	}
	return "toggle(" + t.inner.Name() + ")"
}
// Configure supports "enabled"=bool and forwards cfg to the inner sink.
func (t *ToggleSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["enabled"].(bool); ok {
		t.enabled = v
	}
	if t.inner != nil {
		_ = t.inner.Configure(cfg)
	}
	return nil
}
func (t *ToggleSink) Write(entry *domain.LogEntry) error {
	if entry == nil || !t.enabled || t.inner == nil {
		return nil
	}
	return t.inner.Write(entry)
}

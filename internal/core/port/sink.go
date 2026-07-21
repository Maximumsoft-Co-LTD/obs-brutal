// Package port defines hexagonal ports for the core logging domain
package port

import "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"

// Sink is the output port for log entries.
type Sink interface {
	Write(entry *domain.LogEntry) error
	Close() error
	Name() string
	Health() error
	Configure(config map[string]interface{}) error
}

// SinkBase provides no-op implementations for optional Sink methods.
// Embed in sinks to avoid repeating trivial methods.
type SinkBase struct{}

func (SinkBase) Close() error                           { return nil }
func (SinkBase) Health() error                          { return nil }
func (SinkBase) Configure(map[string]interface{}) error { return nil }

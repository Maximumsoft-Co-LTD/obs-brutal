package service

import (
	"sync"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

type memSink struct {
	mu      sync.Mutex
	entries []*domain.LogEntry
}

func (m *memSink) Write(e *domain.LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}
func (m *memSink) Close() error                           { return nil }
func (m *memSink) Name() string                           { return "mem" }
func (m *memSink) Health() error                          { return nil }
func (m *memSink) Configure(map[string]interface{}) error { return nil }

// TestOTelLogBrt_ChainDoesNotMutateReceiver mirrors the strategy-level
// isolation test one wrapper up: this is the exact logger type boeng
// installs as the package default when Config.OTel is set, so mutation
// here contaminated every operation in OTel mode.
func TestOTelLogBrt_ChainDoesNotMutateReceiver(t *testing.T) {
	sink := &memSink{}
	root := NewOTelLogBrtWithProvider(nil, domain.DebugLevel, sink)
	defer root.Stop()

	opA := root.F("op", "a").F("user_id", "u-1")
	opB := root.F("op", "b")

	opA.Info("a done")
	opB.Info("b done")
	root.Stop() // drain the async pipeline before asserting

	if len(sink.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(sink.entries))
	}
	for _, e := range sink.entries {
		if e.Msg == "b done" {
			if _, leaked := e.F["user_id"]; leaked {
				t.Errorf("user_id from op A leaked into op B: %v", e.F)
			}
			if got := e.F["op"]; got != "b" {
				t.Errorf("op B logged op=%v, want b", got)
			}
		}
	}
}

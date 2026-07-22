package strategy

import (
	"sync"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

type isoSink struct {
	mu      sync.Mutex
	entries []*domain.LogEntry
}

func (m *isoSink) Write(e *domain.LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}
func (m *isoSink) Close() error                           { return nil }
func (m *isoSink) Name() string                           { return "mem" }
func (m *isoSink) Health() error                          { return nil }
func (m *isoSink) Configure(map[string]interface{}) error { return nil }

// TestStrategyLogBrt_ChainDoesNotMutateReceiver is the regression test
// for the field-bleed defect: F/Fs/Ctx must derive a new logger, never
// mutate the shared receiver. With in-place mutation, every field ever
// attached (user_id, span_id, ...) leaked into all later operations'
// log lines.
func TestStrategyLogBrt_ChainDoesNotMutateReceiver(t *testing.T) {
	sink := &isoSink{}
	root := NewStrategyLogBrt(domain.DebugLevel, sink)
	defer root.Stop()

	opA := root.F("op", "a").F("user_id", "u-1")
	opB := root.F("op", "b")

	opA.Info("a done")
	opB.Info("b done")
	root.Info("root done")
	root.Stop() // drain before asserting

	if len(sink.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sink.entries))
	}
	byMsg := map[string]*domain.LogEntry{}
	for _, e := range sink.entries {
		byMsg[e.Msg] = e
	}

	b := byMsg["b done"]
	if b == nil {
		t.Fatal("missing 'b done' entry")
	}
	if _, leaked := b.F["user_id"]; leaked {
		t.Errorf("user_id from op A leaked into op B's entry: %v", b.F)
	}
	if got := b.F["op"]; got != "b" {
		t.Errorf("op B entry has op=%v, want b", got)
	}

	r := byMsg["root done"]
	if r == nil {
		t.Fatal("missing 'root done' entry")
	}
	if len(r.F) != 0 {
		t.Errorf("root logger gained fields from derived chains: %v", r.F)
	}
}

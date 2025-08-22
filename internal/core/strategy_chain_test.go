package core

import (
	"sync"
	"testing"

	"obs-brutal/internal/core/domain"
)

// captureSink collects entries for assertions
type captureSink struct {
	SinkBase
	mu      sync.Mutex
	entries []*domain.LogEntry
}

func (s *captureSink) Write(entry *domain.LogEntry) error {
	s.mu.Lock()
	s.entries = append(s.entries, entry)
	s.mu.Unlock()
	return nil
}

func (s *captureSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *captureSink) Name() string { return "capture" }

// blockFilter drops logs when field "block" is true
type blockFilter struct{}

func (b *blockFilter) ShouldLog(entry *domain.LogEntry) bool {
	if v, ok := entry.Fields["block"].(bool); ok && v {
		return false
	}
	return true
}
func (b *blockFilter) Name() string                           { return "test_block_filter" }
func (b *blockFilter) Configure(map[string]interface{}) error { return nil }

func TestStrategyChainPreservesDerivedType(t *testing.T) {
	sink := &captureSink{}
	l := NewStrategyLogBrt(INFO, sink)
	// add a filter that blocks when field set
	l.AddFilter(&blockFilter{})

	// chain with F() should preserve StrategyLogBrt so filter applies
	chained := l.F("block", true)
	chained.Info("should be filtered out")

	// flush async
	l.Stop()

	if got := sink.Count(); got != 0 {
		t.Fatalf("expected 0 entries due to filter after chain, got %d", got)
	}
}

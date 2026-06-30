package security

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"sync"
	"testing"
	"time"
)

// memSink captures the last log entry for assertions
type memSink struct {
	port.SinkBase
	mu   sync.Mutex
	last *domain.LogEntry
}

func (m *memSink) Write(e *domain.LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = e
	return nil
}
func (m *memSink) Name() string { return "mem" }

func (m *memSink) Last() *domain.LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.last
}

func TestSecurityLogWithSecurityDoesNotClearOriginalFields(t *testing.T) {
	ms := &memSink{}
	sl, err := NewSecurityLogBrt("svc", "1.0.0", "test", "", domain.InfoLevel, ms)
	if err != nil {
		t.Fatalf("failed to create security logger: %v", err)
	}

	base := sl.OTelLogBrt.F("original", "keep").F("user_id", "u1")

	sl.LogWithSecurity(domain.InfoLevel, "secure msg", "user", "user", true)

	base.Info("after")
	time.Sleep(150 * time.Millisecond)
	if ms.Last() == nil {
		t.Fatalf("no log captured from base")
	}
	if v, ok := ms.Last().F["original"]; !ok || v != "keep" {
		t.Fatalf("original field lost or changed, got: %v", v)
	}
}

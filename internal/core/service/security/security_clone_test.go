package security

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"testing"
	"time"
)

// memSink captures the last log entry for assertions
type memSink struct {
	port.SinkBase
	last *domain.LogEntry
}

func (m *memSink) Write(e *domain.LogEntry) error { m.last = e; return nil }
func (m *memSink) Name() string                   { return "mem" }

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
	if ms.last == nil {
		t.Fatalf("no log captured from base")
	}
	if v, ok := ms.last.F["original"]; !ok || v != "keep" {
		t.Fatalf("original field lost or changed, got: %v", v)
	}
}

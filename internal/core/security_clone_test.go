package core

import (
	"testing"
)

func TestSecurityLogWithSecurityDoesNotClearOriginalFields(t *testing.T) {
	// compose SecurityLogBrt without initializing OTEL provider to avoid schema conflicts
	baseStrategy := NewStrategyLogBrt(INFO, NewFastStdoutSink())
	otel := &OTelLogBrt{StrategyLogBrt: baseStrategy, otelProvider: nil}
	sl := &SecurityLogBrt{
		OTelLogBrt:      otel,
		piiMasker:       NewPIIMasker(),
		accessControl:   NewAccessControlManager(),
		auditTrail:      NewAuditTrail(1000),
		securityEnabled: true,
	}

	// add original fields to base logger
	base := sl.F("original", "keep").F("user_id", "u1")

	// call LogWithSecurity which previously cleared fields
	// user authenticated, role user
	if s, ok := base.(*SecurityLogBrt); ok {
		s.LogWithSecurity(INFO, "msg", "user", "user", true)

		// original field must remain on base logger state
		if _, exists := s.fields["original"]; !exists {
			t.Fatalf("original fields cleared by LogWithSecurity")
		}
	} else {
		t.Fatalf("type assertion to *SecurityLogBrt failed")
	}
}

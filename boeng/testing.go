package boeng

import "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"

// Sink is the output port boeng writes structured log entries to.
// Most application code never references this type directly — the
// configured Stdout / Loki / OTel sinks happen automatically.
//
// Sink is exposed so tests can substitute an in-memory implementation
// via SetSinkForTest and verify the operation events their code
// produces.
type Sink = port.Sink

// SetSinkForTest replaces every sink Init would build with the given
// one so tests can capture LogEntry records directly. It returns a
// reset func; tests should defer it so the override doesn't leak into
// subsequent tests.
//
// This helper is intended for tests that verify boeng instrumentation
// in your code. Production callers must not invoke it: the override
// disables stdout/Loki/OTel sinks until the reset func runs.
//
//	sink := &captureSink{} // your test-side port.Sink
//	defer boeng.SetSinkForTest(sink)()
//	boeng.Init(boeng.Config{Service: "x"})
//	// ... drive code under test ...
//	// inspect sink.Entries() ...
func SetSinkForTest(s Sink) func() {
	prev := testSinkOverride
	testSinkOverride = s
	return func() { testSinkOverride = prev }
}

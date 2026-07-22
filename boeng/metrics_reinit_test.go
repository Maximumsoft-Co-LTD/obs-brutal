package boeng

import (
	"testing"
)

// TestConfigureMetricsResetsCache: Init installs a fresh global
// MeterProvider, so the per-op/-event instrument caches must be dropped
// on each Init. Otherwise a second Init leaves cached instruments bound
// to the previous (now shut-down) provider and their metrics silently
// vanish.
func TestConfigureMetricsResetsCache(t *testing.T) {
	// Seed the caches as if ops had fired under a first Init.
	_ = opMetricsFor("seed_op")
	_ = eventMetricsFor("seed_evt")

	metricsMu.RLock()
	seededOps := len(opMetricsCache)
	seededEvts := len(evtMetricsCache)
	metricsMu.RUnlock()
	if seededOps == 0 || seededEvts == 0 {
		t.Fatal("setup: caches should be non-empty after firing an op/event")
	}

	// A subsequent Init must clear them.
	configureMetrics(Config{Service: "reinit", Env: "test"})

	metricsMu.RLock()
	ops := len(opMetricsCache)
	evts := len(evtMetricsCache)
	metricsMu.RUnlock()
	if ops != 0 || evts != 0 {
		t.Fatalf("configureMetrics did not reset caches: ops=%d evts=%d (want 0/0)", ops, evts)
	}
}

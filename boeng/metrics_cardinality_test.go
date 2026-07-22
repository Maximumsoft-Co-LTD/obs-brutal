package boeng

import (
	"fmt"
	"testing"
)

// TestOpMetricNameCardinalityFailsClosed is the name-cardinality half of
// G3: a caller that mints an unbounded number of distinct op names (e.g.
// an HTTP adapter naming ops after raw request paths with embedded ids)
// must not grow the metric cache — and therefore the exported metric
// series — without limit. Past the cap, names route to a shared overflow
// set.
func TestOpMetricNameCardinalityFailsClosed(t *testing.T) {
	metricsMu.Lock()
	savedOp := opMetricsCache
	savedOverflow := opOverflow
	opMetricsCache = map[string]*opMetricSet{}
	opOverflow = nil
	metricsMu.Unlock()
	t.Cleanup(func() {
		metricsMu.Lock()
		opMetricsCache = savedOp
		opOverflow = savedOverflow
		metricsMu.Unlock()
	})

	// Simulate a raw-path HTTP middleware under an id flood.
	for i := 0; i < maxDistinctMetricNames*4; i++ {
		_ = opMetricsFor(fmt.Sprintf("GET /users/%d", i))
	}

	metricsMu.RLock()
	size := len(opMetricsCache)
	overflow := opOverflow
	metricsMu.RUnlock()

	if size > maxDistinctMetricNames {
		t.Fatalf("op metric cache grew past the cap: %d entries (cap %d)", size, maxDistinctMetricNames)
	}
	if overflow == nil {
		t.Fatal("overflow set was never created despite exceeding the cap")
	}

	// A name seen after the cap must resolve to the shared overflow set,
	// not a freshly minted one.
	got := opMetricsFor("GET /users/does-not-fit")
	if got != overflow {
		t.Fatal("post-cap op name did not route to the shared overflow set")
	}
}

// TestEventMetricNameCardinalityFailsClosed mirrors the op test for the
// Emit path.
func TestEventMetricNameCardinalityFailsClosed(t *testing.T) {
	metricsMu.Lock()
	savedEvt := evtMetricsCache
	savedOverflow := evtOverflow
	evtMetricsCache = map[string]*eventMetricSet{}
	evtOverflow = nil
	metricsMu.Unlock()
	t.Cleanup(func() {
		metricsMu.Lock()
		evtMetricsCache = savedEvt
		evtOverflow = savedOverflow
		metricsMu.Unlock()
	})

	for i := 0; i < maxDistinctMetricNames*4; i++ {
		_ = eventMetricsFor(fmt.Sprintf("evt_%d", i))
	}

	metricsMu.RLock()
	size := len(evtMetricsCache)
	overflow := evtOverflow
	metricsMu.RUnlock()

	if size > maxDistinctMetricNames {
		t.Fatalf("event metric cache grew past the cap: %d entries (cap %d)", size, maxDistinctMetricNames)
	}
	if overflow == nil {
		t.Fatal("event overflow set was never created despite exceeding the cap")
	}
}

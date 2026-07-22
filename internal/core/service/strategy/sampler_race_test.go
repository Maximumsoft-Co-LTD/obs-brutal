package strategy

import (
	"sync"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

// TestRateSampler_ConcurrentShouldSample is the regression test for the
// data race on the OTel logging hot path: Manager.ProcessEntry calls
// ShouldSample under only an RLock, so many goroutines invoke it at
// once. A *rand.Rand is not safe for concurrent use, so the sampler
// must guard its own state. Run under -race.
func TestRateSampler_ConcurrentShouldSample(t *testing.T) {
	s := NewRateSampler(0.5) // mid-range forces the rng to be consulted
	e := &domain.LogEntry{Level: domain.InfoLevel, Msg: "x"}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_ = s.ShouldSample(e)
			}
		}()
	}
	wg.Wait()
}

// TestRateSampler_ExtremesDeterministic documents the short-circuit: a
// rate of 1 always samples, 0 never does, without consulting the rng.
func TestRateSampler_ExtremesDeterministic(t *testing.T) {
	e := &domain.LogEntry{Level: domain.InfoLevel, Msg: "x"}
	always := NewRateSampler(1)
	never := NewRateSampler(0)
	for i := 0; i < 100; i++ {
		if !always.ShouldSample(e) {
			t.Fatal("rate 1 must always sample")
		}
		if never.ShouldSample(e) {
			t.Fatal("rate 0 must never sample")
		}
	}
}

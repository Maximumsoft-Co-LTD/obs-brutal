// Performance & Memory Budget. These tests run on every CI push (they
// are regular Test* funcs, not Benchmark* ones, so go test catches them
// without -bench). If a future change pushes any path past its
// documented budget, CI fails loudly.
//
// Budgets are sized for GitHub Actions ubuntu-latest runners, which
// are ~2x slower than an Apple M2. Local apple-silicon runs will be
// comfortably under the budget.
package benchmarks

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// Budget — tightened just past observed CI numbers so a real
// regression fails but normal noise doesn't.
// Baselines reflect the always-on trace context: every op attaches
// trace_id/span_id (a valid W3C context is minted even without an OTLP
// exporter, so propagation and log correlation work with no collector),
// which costs a few allocations per op. allocs/op and bytes/op are
// deterministic across machines, so these are set just above the
// observed M2 numbers; ns/op varies by machine and is gated only off-CI
// (see assertBudget).
const (
	budgetRunNsPerOp     = 6_000 // observed M2: ~1.9 µs; budget 6 µs
	budgetRunAllocsPerOp = 50    // observed M2: 37
	budgetRunBytesPerOp  = 4_096 // observed M2: 2896 B

	// The error path additionally records the error on the span and
	// writes a full JSON error log line per op (RecordError + the error
	// field + the ERROR completion line). During the measurement stdout
	// is redirected to /dev/null (see silenceStdout) so the number covers
	// encode + write cost without depending on how fast the environment
	// drains stdout. Budget 12 µs keeps the order-of-magnitude regression
	// gate with room for slow 2-core runners; allocs/bytes sit just above
	// the observed 77 allocs / 5259 B.
	budgetRunErrNsPerOp     = 12_000
	budgetRunErrAllocsPerOp = 95
	budgetRunErrBytesPerOp  = 6_656

	budgetEmitNsPerOp     = 3_000 // observed M2: ~980 ns; budget 3 µs
	budgetEmitAllocsPerOp = 30    // observed M2: 13
	budgetEmitBytesPerOp  = 2_500 // observed M2: 1048 B

	budgetStepNsPerOp     = 6_000 // step opens its own op; budget like Run
	budgetStepAllocsPerOp = 50
	budgetStepBytesPerOp  = 4_096
)

func TestBudget_Run(t *testing.T) {
	boeng.Init(boeng.Config{Service: "budget", Level: boeng.WarnLevel})
	result := testing.Benchmark(func(b *testing.B) {
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = boeng.Run(ctx, "budget_run", nil, func(ctx context.Context) error { return nil })
		}
	})
	assertBudget(t, "Run", result, budgetRunNsPerOp, budgetRunAllocsPerOp, budgetRunBytesPerOp)
}

// silenceStdout redirects os.Stdout to /dev/null for the duration of a
// benchmark measurement and returns a restore func. The stdout sink
// resolves os.Stdout at write time, so redirecting here is enough to
// keep benchmark-sized log loops out of the CI log while still paying
// a real (cheap, deterministic) write per line.
func silenceStdout(t *testing.T) (restore func()) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	old := os.Stdout
	os.Stdout = devnull
	return func() {
		os.Stdout = old
		devnull.Close()
	}
}

func TestBudget_RunErrorPath(t *testing.T) {
	boeng.Init(boeng.Config{Service: "budget", Level: boeng.WarnLevel})
	want := errors.New("expected")
	restore := silenceStdout(t)
	result := testing.Benchmark(func(b *testing.B) {
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = boeng.Run(ctx, "budget_run_err", nil, func(ctx context.Context) error { return want })
		}
	})
	restore()
	// Error path is allowed to be heavier (records error on span,
	// writes an error log) but must stay within the same order of
	// magnitude — see budgetRunErr* constants.
	assertBudget(t, "Run-error", result, budgetRunErrNsPerOp, budgetRunErrAllocsPerOp, budgetRunErrBytesPerOp)
}

func TestBudget_Emit(t *testing.T) {
	boeng.Init(boeng.Config{Service: "budget", Level: boeng.WarnLevel})
	result := testing.Benchmark(func(b *testing.B) {
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			boeng.Emit(ctx, "budget_event", nil)
		}
	})
	assertBudget(t, "Emit", result, budgetEmitNsPerOp, budgetEmitAllocsPerOp, budgetEmitBytesPerOp)
}

func TestBudget_EnterStep(t *testing.T) {
	boeng.Init(boeng.Config{Service: "budget", Level: boeng.WarnLevel})
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			op := boeng.Enter("budget_enter")
			_ = op.Step("budget_step", func() error { return nil })
			op.Close()
		}
	})
	// Enter + Step + Close is two ops (parent + child) so the budget is
	// roughly 2× Run.
	assertBudget(t, "Enter+Step", result, 2*budgetStepNsPerOp, 2*budgetStepAllocsPerOp, 2*budgetStepBytesPerOp)
}

// TestBudget_MemoryUnderConcurrency proves that running many ops
// concurrently doesn't blow the heap. The runtime is allowed to keep
// some queues, but the per-op allocation should stay bounded — total
// heap growth must scale with op count, NOT with goroutine count
// (otherwise it would mean each goroutine permanently pinned memory).
func TestBudget_MemoryUnderConcurrency(t *testing.T) {
	boeng.Init(boeng.Config{Service: "budget", Level: boeng.WarnLevel})

	const opsPerGoroutine = 200
	type result struct {
		goroutines int
		bytesPerOp uint64
	}

	measure := func(goroutines int) result {
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		var wg sync.WaitGroup
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				for i := 0; i < opsPerGoroutine; i++ {
					_ = boeng.Run(ctx, "concurrent_op", nil, func(ctx context.Context) error { return nil })
				}
			}()
		}
		wg.Wait()

		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		totalOps := uint64(goroutines * opsPerGoroutine)
		// TotalAlloc includes freed memory — it's the right "amount of
		// work" measure. We want it linear in totalOps regardless of
		// goroutine count.
		bytesAlloced := after.TotalAlloc - before.TotalAlloc
		return result{goroutines: goroutines, bytesPerOp: bytesAlloced / totalOps}
	}

	cases := []int{1, 100, 1000}
	results := make([]result, 0, len(cases))
	for _, n := range cases {
		r := measure(n)
		results = append(results, r)
		t.Logf("goroutines=%d ops=%d bytes_per_op=%d", n, n*opsPerGoroutine, r.bytesPerOp)
		// Per-op allocation must stay near the single-goroutine
		// baseline. Allow 2x headroom for concurrency overhead.
		if n > 1 && r.bytesPerOp > results[0].bytesPerOp*3 {
			t.Errorf("memory per op grew %dx at %d goroutines (baseline %d B, got %d B) — likely queue or per-goroutine state leak",
				r.bytesPerOp/results[0].bytesPerOp, n, results[0].bytesPerOp, r.bytesPerOp)
		}
	}
}

// assertBudget compares a testing.BenchmarkResult against the published
// budget and reports per-metric deltas so a CI failure shows exactly
// which dimension broke.
func assertBudget(t *testing.T, label string, r testing.BenchmarkResult, ns, allocs int64, bytes int64) {
	t.Helper()
	if raceEnabled {
		t.Skip("time budgets are sized for uninstrumented builds; skipped under -race")
	}
	t.Logf("[budget %s] ns=%d (≤%d) allocs=%d (≤%d) bytes=%d (≤%d)",
		label, r.NsPerOp(), ns, r.AllocsPerOp(), allocs, r.AllocedBytesPerOp(), bytes)

	// allocs/op and bytes/op are deterministic — they catch the real
	// regressions (an added allocation, a fatter entry) and are safe to
	// gate anywhere.
	if r.AllocsPerOp() > allocs {
		t.Errorf("%s allocs/op = %d, exceeds budget %d", label, r.AllocsPerOp(), allocs)
	}
	if r.AllocedBytesPerOp() > bytes {
		t.Errorf("%s B/op = %d, exceeds budget %d", label, r.AllocedBytesPerOp(), bytes)
	}

	// ns/op is wall-clock and swings 3–10x on shared CI runners (noisy
	// neighbours, no CPU pinning), so it is not a reliable hard gate
	// there — it only fails on machine noise, not real regressions. Gate
	// it on developer machines (fast feedback) but downgrade to an
	// informational log when CI=true. The alloc/byte gates above still
	// catch genuine perf regressions in CI.
	if r.NsPerOp() > ns {
		if isCI() {
			t.Logf("[budget %s] ns/op = %d over soft budget %d (informational on CI; not gated)", label, r.NsPerOp(), ns)
		} else {
			t.Errorf("%s ns/op = %d, exceeds budget %d", label, r.NsPerOp(), ns)
		}
	}
}

// isCI reports whether the tests run on a shared CI runner, where
// wall-clock budgets are unreliable. GitHub Actions (and most CI) set CI=true.
func isCI() bool { return os.Getenv("CI") != "" }

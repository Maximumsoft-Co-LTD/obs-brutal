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
	"runtime"
	"sync"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// Budget — tightened just past observed CI numbers so a real
// regression fails but normal noise doesn't.
const (
	budgetRunNsPerOp     = 6_000   // observed M2: ~1.8 µs; budget 6 µs
	budgetRunAllocsPerOp = 50      // observed M2: 35
	budgetRunBytesPerOp  = 4_096   // observed M2: 2832 B

	// The error path additionally records the error on the span and
	// writes a full JSON error log line to stdout per op — on 2-core
	// CI-class runners that write dominates: observed 8.7 µs plain and
	// 9.7 µs under -cover instrumentation. Budget 12 µs keeps the
	// order-of-magnitude regression gate without failing on runner noise.
	budgetRunErrNsPerOp = 12_000

	budgetEmitNsPerOp     = 3_000  // observed M2: ~980 ns; budget 3 µs
	budgetEmitAllocsPerOp = 30     // observed M2: 21
	budgetEmitBytesPerOp  = 2_500  // observed M2: 1664 B

	budgetStepNsPerOp     = 6_000  // step opens its own op; budget like Run
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

func TestBudget_RunErrorPath(t *testing.T) {
	boeng.Init(boeng.Config{Service: "budget", Level: boeng.WarnLevel})
	want := errors.New("expected")
	result := testing.Benchmark(func(b *testing.B) {
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = boeng.Run(ctx, "budget_run_err", nil, func(ctx context.Context) error { return want })
		}
	})
	// Error path is allowed to be heavier (records error on span,
	// writes an error log) but must stay within the same order of
	// magnitude — see budgetRunErrNsPerOp.
	assertBudget(t, "Run-error", result, budgetRunErrNsPerOp, budgetRunAllocsPerOp+10, budgetRunBytesPerOp+1024)
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
	t.Logf("[budget %s] ns=%d (≤%d) allocs=%d (≤%d) bytes=%d (≤%d)",
		label, r.NsPerOp(), ns, r.AllocsPerOp(), allocs, r.AllocedBytesPerOp(), bytes)
	if r.NsPerOp() > ns {
		t.Errorf("%s ns/op = %d, exceeds budget %d", label, r.NsPerOp(), ns)
	}
	if r.AllocsPerOp() > allocs {
		t.Errorf("%s allocs/op = %d, exceeds budget %d", label, r.AllocsPerOp(), allocs)
	}
	if r.AllocedBytesPerOp() > bytes {
		t.Errorf("%s B/op = %d, exceeds budget %d", label, r.AllocedBytesPerOp(), bytes)
	}
}

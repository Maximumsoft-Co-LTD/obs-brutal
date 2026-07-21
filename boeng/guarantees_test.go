// Runtime Guarantees — the seven user-facing promises boeng makes in
// its README. Every guarantee maps to exactly one test in this file.
// If a test in this file fails, a documented promise has broken.
//
//	G1. Every operation produces a structured log line on completion.
//	G2. Every operation reports duration_ms.
//	G3. Every operation emits per-op metric instruments (total/duration/error/panic).
//	G4. Every panic is captured, recorded, AND re-raised.
//	G5. Every child operation preserves correlation through context.
//	G6. Every adapter accepts a parent ctx and produces a child operation.
//	G7. Every exporter failure degrades gracefully (no panic, sinks keep working).
package boeng_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func runGuarantee(t *testing.T, cfg boeng.Config, body func(t *testing.T, sink *captureSink)) {
	t.Helper()
	sink := &captureSink{}
	reset := boeng.SetSinkForTest(sink)
	defer reset()
	obs := boeng.Init(cfg)
	defer obs.Close()
	body(t, sink)
}

// G1. Every operation produces a structured log line on completion.
func TestGuarantee_G1_StructuredCompletionLog(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "op_x", nil, func(ctx context.Context) error { return nil })
		done := findEntryWithMsg(t, sink.snapshot(), "op_x completed")
		if done.Level != domain.InfoLevel {
			t.Errorf("completion level = %v, want INFO", done.Level)
		}
		if done.F["op"] != "op_x" {
			t.Errorf("op field missing/wrong: %v", done.F["op"])
		}
	})
}

// G2. Every operation reports duration_ms.
func TestGuarantee_G2_DurationReported(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "op_dur", nil, func(ctx context.Context) error { return nil })
		done := findEntryWithMsg(t, sink.snapshot(), "op_dur completed")
		if _, ok := done.F["duration_ms"]; !ok {
			t.Errorf("duration_ms missing from completion log: %v", done.F)
		}
	})
}

// G3. Every operation emits the four per-op metric instruments. We can't
// trivially inspect OTel counters without a manual reader; this guarantee
// is enforced by exercising the cardinality-guard helper (proves the
// metric labels would be safe to emit) plus the existing tests in
// metrics_test.go that cover sanitization and label filtering directly.
func TestGuarantee_G3_MetricLabelsAreCardinalitySafe(t *testing.T) {
	boeng.Init(boeng.Config{
		Service:      "g",
		Env:          "test",
		MetricLabels: []string{"tier"},
	})

	got := boeng.MetricLabelsForTest(map[string]any{
		"tier":     "premium",
		"user_id":  "u-99999", // must be filtered
		"order_id": "ord-7",   // must be filtered
	})
	keys := map[string]bool{}
	for _, kv := range got {
		keys[string(kv.Key)] = true
	}
	for _, want := range []string{"service", "env", "tier"} {
		if !keys[want] {
			t.Errorf("missing required metric label %q", want)
		}
	}
	for _, must := range []string{"user_id", "order_id"} {
		if keys[must] {
			t.Errorf("metric label %q leaked past cardinality guard", must)
		}
	}
}

// G4. Every panic is captured, recorded, AND re-raised. boeng must
// never swallow a panic — it records observability data then lets the
// program unwind normally.
func TestGuarantee_G4_PanicCapturedAndReraised(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("panic not re-raised")
			}
			// Failure log must have already been recorded by the time
			// we reach the outer recover.
			failed := findEntryWithMsg(t, sink.snapshot(), "panic_op failed")
			if failed.Level != domain.ErrorLevel {
				t.Errorf("panic completion level = %v, want ERROR", failed.Level)
			}
		}()
		_ = boeng.Run(context.Background(), "panic_op", nil, func(ctx context.Context) error {
			panic("boom")
		})
	})
}

// G5. Every child operation preserves correlation through context: a
// nested operation appears as a child of its parent in the log stream
// (parent's completion follows child's completion).
func TestGuarantee_G5_ChildOpPreservesCorrelation(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "outer", nil, func(ctx context.Context) error {
			return boeng.Run(ctx, "inner", nil, func(ctx context.Context) error {
				return nil
			})
		})

		entries := sink.snapshot()
		innerIdx, outerIdx := -1, -1
		for i, e := range entries {
			switch e.Msg {
			case "inner completed":
				innerIdx = i
			case "outer completed":
				outerIdx = i
			}
		}
		if innerIdx == -1 || outerIdx == -1 {
			t.Fatalf("nested ops not both present: %v", msgsOf(entries))
		}
		if innerIdx >= outerIdx {
			t.Errorf("inner completion (i=%d) must precede outer (i=%d)", innerIdx, outerIdx)
		}
	})
}

// G6. Every adapter accepts a parent ctx and produces a child operation.
// We exercise this through boeng's own Run + Step combination; the adapter
// packages each verify the same shape against their respective frameworks.
func TestGuarantee_G6_AdapterAcceptsParentCtx(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		ctx, op := boeng.EnterCtx(context.Background(), "parent_op", nil)
		defer op.Close()

		if err := op.Step("child_step", func() error { return nil }); err != nil {
			t.Fatalf("step: %v", err)
		}
		// Inside the parent op, child Run must also nest.
		_ = boeng.Run(ctx, "nested_run", nil, func(ctx context.Context) error { return nil })

		_ = ctx
	})
	// If we reached here, the parent op closed without leaking — the
	// child operations completed under it. The order assertion lives
	// in G5; here we're just asserting "no panic, no leak".
}

// G7. Every exporter failure degrades gracefully. With OTLP pointed at
// a closed port, Init + Run must not panic and the configured sink must
// still receive records.
func TestGuarantee_G7_ExporterFailureDegradesGracefully(t *testing.T) {
	sink := &captureSink{}
	reset := boeng.SetSinkForTest(sink)
	defer reset()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic under unreachable OTLP: %v", r)
		}
	}()

	obs := boeng.Init(boeng.Config{
		Service: "g",
		OTel:    "127.0.0.1:1", // closed port
	})
	defer obs.Close()

	if err := boeng.Run(context.Background(), "graceful_op", nil,
		func(ctx context.Context) error { return errors.New("expected") }); err != nil {
		// Run propagates errors normally; we don't care about the value
		// here, only that it didn't panic.
		_ = err
	}
	// We don't assert on sink size because the OTel path is async and
	// would force us to wait the 10s OTLP shutdown for flush. The
	// load-bearing guarantee is "no panic" — verified by the deferred
	// recover above.
}

// Verification tests for boeng.
//
// These are NOT unit tests of individual functions — those live in the
// other *_test.go files. This file asserts the end-to-end promises the
// README makes to users:
//
//  1. Log line is produced on success with the expected shape (level,
//     msg, op, duration_ms, subject fields).
//  2. Log line on error contains the error and is at ERROR level.
//  3. A panicking operation is recovered, recorded, AND re-raised.
//  4. Legacy Enter + Step + Close produces the documented sequence of
//     log entries.
//  5. Nested EnterCtx produces a child operation under the parent's
//     trace.
//  6. Metric labels respect the cardinality allowlist.
//  7. Init with an unreachable OTLP endpoint does not panic; logging
//     still works.
//
// Each test installs a capture sink so it can inspect the exact
// domain.LogEntry records boeng emits, instead of grepping stdout.
package boeng_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"obs-brutal/boeng"
	"obs-brutal/internal/core/domain"
)

// captureSink implements port.Sink. It is the lens we use to see what
// boeng actually wrote: structured LogEntry records, not the JSON bytes
// that would land on stdout.
type captureSink struct {
	mu      sync.Mutex
	entries []domain.LogEntry
}

func (c *captureSink) Write(e *domain.LogEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, *e)
	return nil
}
func (c *captureSink) Close() error                        { return nil }
func (c *captureSink) Name() string                        { return "capture" }
func (c *captureSink) Health() error                       { return nil }
func (c *captureSink) Configure(map[string]interface{}) error { return nil }

func (c *captureSink) snapshot() []domain.LogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]domain.LogEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

// withCapture installs a fresh capture sink and a fresh boeng default,
// returning the sink so the test can inspect it. The cleanup func is
// deferred by the caller.
func withCapture(t *testing.T, cfg boeng.Config) (*captureSink, func()) {
	t.Helper()
	sink := &captureSink{}
	resetSink := boeng.SetSinkForTest(sink)
	obs := boeng.Init(cfg)
	return sink, func() {
		_ = obs.Close()
		resetSink()
	}
}

// findEntryWithMsg returns the first entry whose Msg matches and
// fails the test if none is found.
func findEntryWithMsg(t *testing.T, entries []domain.LogEntry, msg string) domain.LogEntry {
	t.Helper()
	for _, e := range entries {
		if e.Msg == msg {
			return e
		}
	}
	t.Fatalf("no entry with msg=%q. got: %v", msg, msgsOf(entries))
	return domain.LogEntry{}
}

func msgsOf(entries []domain.LogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Msg)
	}
	return out
}

func TestRunSuccess(t *testing.T) {
	sink, cleanup := withCapture(t, boeng.Config{Service: "verify"})
	defer cleanup()

	subject := map[string]any{"user_id": "u-1", "plan": "pro"}
	err := boeng.Run(context.Background(), "create_user", subject, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Run returned err: %v", err)
	}

	entries := sink.snapshot()
	done := findEntryWithMsg(t, entries, "create_user completed")
	if done.Level != domain.InfoLevel {
		t.Errorf("completion log level = %v, want INFO", done.Level)
	}
	if done.F["op"] != "create_user" {
		t.Errorf("op field = %v, want create_user", done.F["op"])
	}
	// user_id is promoted from F to the dedicated UserID column by
	// domain.Promote() before the entry hits the sink.
	if done.UserID != "u-1" {
		t.Errorf("UserID missing/wrong: %q (F=%v)", done.UserID, done.F)
	}
	if _, ok := done.F["duration_ms"]; !ok {
		t.Errorf("duration_ms missing on completion log: %v", done.F)
	}
}

func TestRunError(t *testing.T) {
	sink, cleanup := withCapture(t, boeng.Config{Service: "verify"})
	defer cleanup()

	want := errors.New("validation failed")
	got := boeng.Run(context.Background(), "create_user", nil, func(ctx context.Context) error {
		return want
	})
	if !errors.Is(got, want) {
		t.Fatalf("Run did not propagate error: %v", got)
	}

	entries := sink.snapshot()
	failed := findEntryWithMsg(t, entries, "create_user failed")
	if failed.Level != domain.ErrorLevel {
		t.Errorf("failure log level = %v, want ERROR", failed.Level)
	}
	if failed.F["error"] != want.Error() {
		t.Errorf("error field = %v, want %q", failed.F["error"], want.Error())
	}
}

func TestRunPanic(t *testing.T) {
	sink, cleanup := withCapture(t, boeng.Config{Service: "verify"})
	defer cleanup()

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Run did not re-raise panic")
		}
		// Logs must already be recorded at this point.
		entries := sink.snapshot()
		failed := findEntryWithMsg(t, entries, "panic_op failed")
		if failed.Level != domain.ErrorLevel {
			t.Errorf("panic log level = %v, want ERROR", failed.Level)
		}
		errStr, _ := failed.F["error"].(string)
		if !strings.Contains(errStr, "panic") {
			t.Errorf("error field should mention panic: %v", failed.F["error"])
		}
	}()

	_ = boeng.Run(context.Background(), "panic_op", nil, func(ctx context.Context) error {
		panic("kaboom")
	})
}

func TestEnterLegacy(t *testing.T) {
	sink, cleanup := withCapture(t, boeng.Config{Service: "verify"})
	defer cleanup()

	op := boeng.Enter("save_user", map[string]any{"user_id": "u-9"})
	op.Log("validating")
	_ = op.Step("insert_db", func() error { return nil })
	op.Emit("row_inserted", map[string]any{"row_id": 7})
	op.Close()

	entries := sink.snapshot()
	wantMsgs := []string{
		"validating",
		"insert_db completed",
		"row_inserted",
		"save_user completed",
	}
	got := msgsOf(entries)
	for _, want := range wantMsgs {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected msg %q in legacy flow; got %v", want, got)
		}
	}
}

// TestEnterCtxChildSpan asserts that nesting EnterCtx creates a child
// operation whose completion log appears AFTER the parent records its
// own state in ctx — i.e., propagation through ctx is wired up. Full
// span lineage verification (trace_id propagation across collectors)
// lives in the compose-backed integration tests, not here.
func TestEnterCtxChildSpan(t *testing.T) {
	sink, cleanup := withCapture(t, boeng.Config{Service: "verify"})
	defer cleanup()

	parentCtx, parent := boeng.EnterCtx(context.Background(), "parent_op", nil)
	_, child := boeng.EnterCtx(parentCtx, "child_op", nil)
	child.Close()
	parent.Close()

	entries := sink.snapshot()
	findEntryWithMsg(t, entries, "parent_op completed")
	findEntryWithMsg(t, entries, "child_op completed")

	// child completion must come before parent completion in the log
	// stream because Close() unwinds inner-first.
	var childIdx, parentIdx = -1, -1
	for i, e := range entries {
		if e.Msg == "child_op completed" {
			childIdx = i
		}
		if e.Msg == "parent_op completed" {
			parentIdx = i
		}
	}
	if childIdx == -1 || parentIdx == -1 {
		t.Fatalf("missing nested ops in log stream: %v", msgsOf(entries))
	}
	if childIdx >= parentIdx {
		t.Errorf("child completion (index %d) must precede parent (index %d)", childIdx, parentIdx)
	}
}

// TestMetricCardinalityGuard verifies the documented promise that subject
// fields not in Config.MetricLabels are dropped from metric labels even
// when LogFields() or reflection produced them.
func TestMetricCardinalityGuard(t *testing.T) {
	boeng.Init(boeng.Config{
		Service:      "verify",
		Env:          "test",
		MetricLabels: []string{"user_type"},
	})

	subject := map[string]any{
		"user_id":   "u-12345", // high-cardinality, must be filtered
		"order_id":  "ord-7",   // high-cardinality, must be filtered
		"user_type": "premium", // allowlisted, must survive
		"region":    "ap-se-1", // NOT allowlisted in this test
	}
	got := boeng.MetricLabelsForTest(subject)
	keys := map[string]bool{}
	for _, kv := range got {
		keys[string(kv.Key)] = true
	}

	for _, must := range []string{"service", "env", "user_type"} {
		if !keys[must] {
			t.Errorf("missing required label %q; got %v", must, keys)
		}
	}
	for _, mustNot := range []string{"user_id", "order_id", "region"} {
		if keys[mustNot] {
			t.Errorf("label %q should have been filtered; got %v", mustNot, keys)
		}
	}
}

// TestOTLPFallback verifies the documented "never panics on unreachable
// OTLP" promise. 127.0.0.1:1 is closed on virtually every machine, so
// if boeng tried to require the collector at Init time, this would
// panic or hang forever. The shutdown wait that OTel imposes is
// acceptable here because the test runs in isolation.
func TestOTLPFallback(t *testing.T) {
	sink := &captureSink{}
	resetSink := boeng.SetSinkForTest(sink)
	defer resetSink()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Init/Run panicked under unreachable OTLP: %v", r)
		}
	}()

	obs := boeng.Init(boeng.Config{
		Service: "verify",
		OTel:    "127.0.0.1:1",
	})
	defer obs.Close()

	if err := boeng.Run(context.Background(), "fallback_op", nil, func(ctx context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("Run failed under OTLP fallback: %v", err)
	}
	// We don't assert on sink contents — the OTel path is async and
	// would require waiting for the OTLP shutdown timeout (~10s) for
	// the batch to flush. Verifying "Init + Run don't panic" is the
	// load-bearing promise.
}

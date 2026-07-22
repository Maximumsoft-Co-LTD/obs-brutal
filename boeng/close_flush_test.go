package boeng_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// TestClose_FlushesAsyncPipeline is the public-API regression test for
// the flush-on-close defect: with Async enabled, a short-lived process
// that runs operations and immediately calls Close must not lose log
// entries. Before the fix, AsyncPipeline.Stop cancelled its workers
// without draining, so processes shorter than the flush interval
// (100 ms) emitted nothing at all.
func TestClose_FlushesAsyncPipeline(t *testing.T) {
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()

	obs := boeng.Init(boeng.Config{Service: "flush", Async: true, Level: boeng.DebugLevel})

	const ops = 10
	for i := 0; i < ops; i++ {
		_ = boeng.Run(context.Background(), fmt.Sprintf("flush_op_%d", i), nil,
			func(ctx context.Context) error { return nil })
	}
	if err := obs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Every op logs a start (DEBUG) and a completion (INFO) line.
	got := len(sink.snapshot())
	if want := ops * 2; got != want {
		t.Fatalf("Close dropped async entries: got %d log entries, want %d", got, want)
	}
}

// Cron-style dogfood: a background worker that wakes on a timer,
// processes a batch of jobs, and exits. The pattern is common to
// periodic reconciliation tasks, billing sweeps, and queue drains.
//
// What boeng buys you here:
//   - Each tick is one Operation. boeng.Run wraps the whole tick so
//     duration, errors, and panics are auto-recorded — no try/finally
//     ladder, no manual start/end log pair.
//   - Each job inside a tick is a Step, so the trace splits cleanly
//     ("ten jobs took 200ms, of which job #4 took 180ms because…").
//
// Run:   go run ./examples/cron
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

const tickInterval = 200 * time.Millisecond
const totalTicks = 3

func main() {
	defer boeng.Init(boeng.Config{Service: "cron_demo", Env: "dev"}).Close()

	ctx := context.Background()
	for i := 1; i <= totalTicks; i++ {
		if err := tick(ctx, i); err != nil {
			fmt.Println("tick error:", err)
		}
		time.Sleep(tickInterval)
	}
}

// tick is one cron iteration — boeng treats it as a top-level operation.
func tick(ctx context.Context, iteration int) error {
	return boeng.Run(ctx, "reconcile_tick", map[string]any{"iteration": iteration},
		func(ctx context.Context) error {
			_, op := boeng.EnterCtx(ctx, "process_batch", nil)
			defer op.Close()

			// Three jobs per tick. Job #2 of tick #2 fails on purpose so
			// the demo shows ERROR-level completion cascading to the op.
			for j := 1; j <= 3; j++ {
				jobID := fmt.Sprintf("job-%d-%d", iteration, j)
				err := op.Step("run_job", func() error {
					return processJob(jobID, iteration == 2 && j == 2)
				})
				if err != nil {
					// Continue the batch — the failing step is already
					// recorded as ERROR; the batch op will mark failed
					// at Close because Step cascaded the error.
					_ = err
				}
			}
			return nil
		},
	)
}

func processJob(id string, shouldFail bool) error {
	time.Sleep(20 * time.Millisecond)
	if shouldFail {
		return errors.New("simulated job failure for " + id)
	}
	return nil
}

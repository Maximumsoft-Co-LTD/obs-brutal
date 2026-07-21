package benchmarks

import (
	"context"
	"errors"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

func init() {
	boeng.Init(boeng.Config{Service: "bench", Level: boeng.WarnLevel})
}

// BenchmarkRun measures the cost of wrapping a no-op closure with the full
// pipeline (span, log, metrics, panic recovery).
func BenchmarkRun(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = boeng.Run(ctx, "noop", nil, func(ctx context.Context) error {
			return nil
		})
	}
}

// BenchmarkRunWithSubject adds the cost of reflection over a struct
// subject (skipping zero values, snake_case naming, sensitive-key check).
func BenchmarkRunWithSubject(b *testing.B) {
	ctx := context.Background()
	subj := struct {
		UserID string
		Tier   string
		Plan   string
	}{"u-1", "premium", "annual"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = boeng.Run(ctx, "with_subject", subj, func(ctx context.Context) error {
			return nil
		})
	}
}

// BenchmarkRunErrorPath shows the cost when fn returns an error — span
// gets RecordError, completion log goes to ERROR, _error_total bumps.
func BenchmarkRunErrorPath(b *testing.B) {
	ctx := context.Background()
	want := errors.New("expected")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = boeng.Run(ctx, "err_op", nil, func(ctx context.Context) error {
			return want
		})
	}
}

// BenchmarkEnterStep mirrors the legacy/imperative usage pattern.
func BenchmarkEnterStep(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := boeng.Enter("bench_op")
		_ = op.Step("phase_a", func() error { return nil })
		_ = op.Step("phase_b", func() error { return nil })
		op.Close()
	}
}

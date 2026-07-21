// Comparative benchmark. The boeng API is intentionally heavier than
// raw logging — it opens a span, captures duration, runs metrics,
// recovers panics. The honest question is "how much heavier?"
//
// This file runs the same payload (one structured log line, one wrapped
// operation) through three implementations:
//
//	stdlib  : log.Printf with fmt.Sprintf
//	slog    : log/slog.LogAttrs with the JSON handler
//	boeng   : boeng.Run with a Loggable subject
//
// All three write to io.Discard so the cost being measured is the
// formatting / dispatch path, not the sink. Run:
//
//	go test -bench=Comparison -benchmem -benchtime=2s ./benchmarks/
package benchmarks

import (
	"context"
	"io"
	"log"
	"log/slog"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

var (
	stdLogger    = log.New(io.Discard, "", log.LstdFlags)
	slogJSON     = slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	boengSubject = comparisonUser{ID: "u-1", Tier: "premium", Plan: "annual"}
)

type comparisonUser struct {
	ID   string
	Tier string
	Plan string
}

func (u comparisonUser) LogFields() map[string]any {
	return map[string]any{
		"user_id": u.ID,
		"tier":    u.Tier,
		"plan":    u.Plan,
	}
}

// BenchmarkComparison_Stdlib measures the cheapest baseline — a single
// log.Printf with three field interpolations. Produces no structured
// output, no span, no metric.
func BenchmarkComparison_Stdlib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		stdLogger.Printf("op=%s user_id=%s tier=%s plan=%s", "create_user",
			boengSubject.ID, boengSubject.Tier, boengSubject.Plan)
	}
}

// BenchmarkComparison_Slog measures stdlib's log/slog with structured
// attributes routed through the JSON handler. Produces structured
// output but no span, no duration tracking, no metric.
func BenchmarkComparison_Slog(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		slogJSON.LogAttrs(ctx, slog.LevelInfo, "create_user",
			slog.String("user_id", boengSubject.ID),
			slog.String("tier", boengSubject.Tier),
			slog.String("plan", boengSubject.Plan),
		)
	}
}

// BenchmarkComparison_BoengRun measures the full boeng.Run pipeline:
// open span, extract subject via LogFields, emit start + completion
// log lines, record duration histogram, bump per-op counters, recover
// panic (none here). What you pay for is what you get.
func BenchmarkComparison_BoengRun(b *testing.B) {
	boeng.Init(boeng.Config{Service: "bench", Level: boeng.WarnLevel})
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = boeng.Run(ctx, "create_user", boengSubject, func(ctx context.Context) error {
			return nil
		})
	}
}

// BenchmarkComparison_BoengEmit measures the cheapest boeng path —
// a one-shot domain event (no enclosing operation). Closest apples-
// to-apples comparison with slog.LogAttrs since neither opens a span.
func BenchmarkComparison_BoengEmit(b *testing.B) {
	boeng.Init(boeng.Config{Service: "bench", Level: boeng.WarnLevel})
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		boeng.Emit(ctx, "user_created", boengSubject)
	}
}

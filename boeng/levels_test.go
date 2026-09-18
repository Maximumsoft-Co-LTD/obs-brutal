package boeng_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func hasMsg(entries []domain.LogEntry, msg string) bool {
	for _, e := range entries {
		if e.Msg == msg {
			return true
		}
	}
	return false
}

// Config.QuietOps moves the success-path "<op> completed" line to another
// level so a production deployment can run at Level=INFO (events visible)
// while the per-operation chatter is demoted to DEBUG and filtered out.
func TestConfig_QuietOps_DemotesCompletionLineOnly(t *testing.T) {
	cfg := boeng.Config{Service: "g", Level: boeng.InfoLevel, QuietOps: true}
	runGuarantee(t, cfg, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "quiet_op", nil, func(ctx context.Context) error {
			boeng.Emit(ctx, "quiet_event", nil)
			return nil
		})
		got := sink.snapshot()
		if hasMsg(got, "quiet_op completed") {
			t.Errorf("QuietOps completion line should be DEBUG and filtered at Level=INFO; got %v", got)
		}
		if !hasMsg(got, "quiet_event") {
			t.Errorf("Emit at INFO must still be visible when only QuietOps is set; got %v", got)
		}
	})
}

func TestConfig_QuietOps_DoesNotHideFailures(t *testing.T) {
	cfg := boeng.Config{Service: "g", Level: boeng.InfoLevel, QuietOps: true}
	runGuarantee(t, cfg, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "bad_op", nil, func(ctx context.Context) error {
			return errors.New("boom")
		})
		failed := findEntryWithMsg(t, sink.snapshot(), "bad_op failed")
		if failed.Level != domain.ErrorLevel {
			t.Errorf("failure level = %v, want ERROR regardless of QuietOps", failed.Level)
		}
	})
}

func TestConfig_QuietOps_ZeroKeepsInfoDefault(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "dflt_op", nil, func(ctx context.Context) error { return nil })
		done := findEntryWithMsg(t, sink.snapshot(), "dflt_op completed")
		if done.Level != domain.InfoLevel {
			t.Errorf("default completion level = %v, want INFO", done.Level)
		}
	})
}

// Config.EmitLevel lets a deployment that runs at Level=WARN keep its
// per-request summary events (the case that emptied a dashboard).
func TestConfig_EmitLevel_SurvivesWarnFilter(t *testing.T) {
	cfg := boeng.Config{Service: "g", Level: boeng.WarnLevel, EmitLevel: boeng.WarnLevel}
	runGuarantee(t, cfg, func(t *testing.T, sink *captureSink) {
		_ = boeng.Run(context.Background(), "warn_op", nil, func(ctx context.Context) error {
			boeng.Emit(ctx, "summary_event", map[string]any{"k": "v"})
			return nil
		})
		got := sink.snapshot()
		ev := findEntryWithMsg(t, got, "summary_event")
		if ev.Level != domain.WarnLevel {
			t.Errorf("emit level = %v, want WARN", ev.Level)
		}
		if ev.F["event"] != "summary_event" || ev.F["k"] != "v" {
			t.Errorf("emit fields lost: %v", ev.F)
		}
		if hasMsg(got, "warn_op completed") {
			t.Errorf("INFO completion line must still be filtered at Level=WARN")
		}
	})
}

func TestConfig_EmitLevel_AppliesToOpEmit(t *testing.T) {
	cfg := boeng.Config{Service: "g", Level: boeng.WarnLevel, EmitLevel: boeng.WarnLevel}
	runGuarantee(t, cfg, func(t *testing.T, sink *captureSink) {
		op := boeng.Enter("handle_op")
		op.Emit("handle_event", nil)
		op.Close()
		ev := findEntryWithMsg(t, sink.snapshot(), "handle_event")
		if ev.Level != domain.WarnLevel {
			t.Errorf("op.Emit level = %v, want WARN", ev.Level)
		}
	})
}

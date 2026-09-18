package boeng_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// driveAllOutcomes runs one ok op, one failing op, one panicking op and
// one event under the given op-name prefix.
func driveAllOutcomes(prefix string) {
	ctx := context.Background()
	_ = boeng.Run(ctx, prefix+"_ok", nil, func(ctx context.Context) error {
		boeng.Emit(ctx, prefix+"_event", nil)
		return nil
	})
	_ = boeng.Run(ctx, prefix+"_bad", nil, func(context.Context) error { return errors.New("boom") })
	func() {
		defer func() { _ = recover() }()
		_ = boeng.Run(ctx, prefix+"_boom", nil, func(context.Context) error { panic("kaboom") })
	}()
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("/metrics missing %q", w)
		}
	}
}

func mustNotContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if strings.Contains(body, w) {
			t.Errorf("/metrics should not contain %q", w)
		}
	}
}

// LabeledMetrics: one fixed instrument family, op and outcome as labels,
// seconds as unit — `sum by (op)` works, no series minted per op name.
func TestMetricSchema_Labeled(t *testing.T) {
	clearOTLPEnv(t)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "lab", Env: "test", MetricSchema: boeng.LabeledMetrics})
	defer obs.Close()
	h := obs.MetricsHandler()

	driveAllOutcomes("lab")
	body := scrape(t, h)

	mustContain(t, body,
		"boeng_operation_duration_seconds_bucket{",
		"boeng_operation_duration_seconds_count{",
		`op="lab_ok"`, `outcome="ok"`,
		`op="lab_bad"`, `outcome="error"`,
		`op="lab_boom"`, `outcome="panic"`,
		"boeng_events_total{", `event="lab_event"`,
		`service="lab"`, `env="test"`,
		`le="0.001"`, `le="10"`,
	)
	mustNotContain(t, body,
		"lab_ok_total", "lab_ok_duration_ms", "lab_bad_error_total", "lab_boom_panic_total", "lab_event_total",
		"_seconds_seconds", "boeng_events_total_total",
	)
}

// Default (PerOpMetrics) is unchanged: legacy names only.
func TestMetricSchema_DefaultIsPerOp(t *testing.T) {
	clearOTLPEnv(t)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "dflt"})
	defer obs.Close()
	h := obs.MetricsHandler()

	driveAllOutcomes("dflt")
	body := scrape(t, h)
	mustContain(t, body, "dflt_ok_total", "dflt_ok_duration_ms_bucket", "dflt_bad_error_total", "dflt_boom_panic_total", "dflt_event_total")
	mustNotContain(t, body, "boeng_operation_duration_seconds", "boeng_events_total")
}

// BothMetrics emits the legacy and the labeled family side by side for a
// dashboard migration window.
func TestMetricSchema_Both(t *testing.T) {
	clearOTLPEnv(t)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "both", MetricSchema: boeng.BothMetrics})
	defer obs.Close()
	h := obs.MetricsHandler()

	driveAllOutcomes("both")
	body := scrape(t, h)
	mustContain(t, body, "both_ok_total", "both_ok_duration_ms_bucket", `op="both_ok"`, "boeng_operation_duration_seconds_bucket{", "boeng_events_total{", `event="both_event"`)
}

// The op label value is the sanitized op name, same as the legacy metric
// name, and the distinct-name cap still applies (G3 name-cardinality).
func TestMetricSchema_Labeled_OpValueIsSanitized(t *testing.T) {
	clearOTLPEnv(t)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "san", MetricSchema: boeng.LabeledMetrics})
	defer obs.Close()
	h := obs.MetricsHandler()

	_ = boeng.Run(context.Background(), "GET /users/:id", nil, func(context.Context) error { return nil })
	body := scrape(t, h)
	mustContain(t, body, `op="get_users_id"`)
	mustNotContain(t, body, `op="GET /users/:id"`)
}

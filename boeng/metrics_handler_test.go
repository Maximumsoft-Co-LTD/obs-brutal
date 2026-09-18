package boeng_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

func scrape(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d", rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	return string(b)
}

// Obs.MetricsHandler exposes the in-process Prometheus registry so a
// deployment whose OTLP collector does not accept metrics still has a
// pull path. Calling it with no OTLP endpoint configured must also turn
// metric recording on — an empty /metrics page is a silent failure.
func TestObs_MetricsHandler_ServesPerOpMetrics(t *testing.T) {
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "mh", Env: "test"})
	defer obs.Close()

	h := obs.MetricsHandler()
	if h == nil {
		t.Fatal("MetricsHandler returned nil")
	}
	_ = boeng.Run(context.Background(), "scrape_op", nil, func(ctx context.Context) error {
		boeng.Emit(ctx, "scrape_event", nil)
		return nil
	})

	body := scrape(t, h)
	for _, want := range []string{
		"scrape_op_total",
		"scrape_op_duration_ms_bucket",
		"scrape_event_total",
		`service="mh"`,
		`env="test"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics missing %q\n---\n%s", want, body)
		}
	}
	// Item 5 of the hash-central feedback: the documented histogram name
	// is <op>_duration_ms. The exporter must not append a unit suffix.
	if strings.Contains(body, "_duration_ms_milliseconds") {
		t.Errorf("histogram name carries a unit suffix; want <op>_duration_ms_bucket only\n---\n%s", body)
	}
}

func TestObs_MetricsHandler_NilObsIsSafe(t *testing.T) {
	var obs *boeng.Obs
	h := obs.MetricsHandler()
	if h == nil {
		t.Fatal("nil Obs must still return a usable handler")
	}
	_ = scrape(t, h)
}

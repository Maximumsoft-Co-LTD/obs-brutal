package boeng_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// appMeterProvider stands in for a service that owns its metrics pipeline
// (SDK MeterProvider + its own reader/exporter) and installed it globally
// before boeng.Init.
func appMeterProvider(t *testing.T) (*sdkmetric.MeterProvider, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() {
		otel.SetMeterProvider(metricnoop.NewMeterProvider())
		_ = mp.Shutdown(context.Background())
	})
	return mp, reader
}

func instrumentNames(t *testing.T, reader *sdkmetric.ManualReader) []string {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names = append(names, m.Name)
		}
	}
	return names
}

// The application owns the global MeterProvider: boeng's per-op metrics
// must flow into it (one pipeline), and MetricsHandler must refuse to
// replace it — serving an explicit non-200 rather than an empty page.
func TestObs_MetricsHandler_NeverReplacesAppMeterProvider(t *testing.T) {
	clearOTLPEnv(t)
	app, reader := appMeterProvider(t)
	appTP, _ := appTracerProvider(t)
	otel.SetTracerProvider(appTP)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "meter-adopt"})
	defer obs.Close()

	h := obs.MetricsHandler()
	if otel.GetMeterProvider() != app {
		t.Fatal("MetricsHandler replaced the application's MeterProvider")
	}
	_ = boeng.Run(context.Background(), "meter_probe", nil, func(ctx context.Context) error { return nil })

	names := instrumentNames(t, reader)
	if !contains(names, "meter_probe_total") {
		t.Errorf("boeng metrics did not reach the app's MeterProvider; got %v", names)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code == http.StatusOK {
		t.Errorf("handler answered 200 with nothing to serve; want an explicit non-200, body: %q", rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "meterprovider") {
		t.Errorf("body should explain where the metrics went: %q", rec.Body.String())
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

package boeng_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// fakeCollector serves a gRPC endpoint with no OTLP services registered.
// Exporters get codes.Unimplemented — non-retryable — so Close returns
// immediately instead of waiting out the 5s flush deadline against a
// closed port. It is also exactly the "Alloy answers Unimplemented for
// MetricsService" shape from the adoption report.
func fakeCollector(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func clearOTLPEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"} {
		t.Setenv(k, "")
	}
}

// recording reports whether the span boeng opened for an op is recording.
// Exporting mode samples AlwaysSample (recording); the no-collector mode
// samples NeverSample (valid ids, not recording). It is the one
// observable that tells the two modes apart from outside the package.
func recording(t *testing.T) bool {
	t.Helper()
	var rec bool
	_ = boeng.Run(context.Background(), "probe", nil, func(ctx context.Context) error {
		rec = trace.SpanFromContext(ctx).IsRecording()
		return nil
	})
	return rec
}

// Config.OTel empty + OTEL_EXPORTER_OTLP_ENDPOINT set: boeng exports,
// letting the SDK read the endpoint (and headers, timeout, certs) from
// the environment — the OTel-spec way to configure a service.
func TestInit_EnvEndpointEnablesExport(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+fakeCollector(t))
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "envexp"})
	defer obs.Close()

	if !recording(t) {
		t.Error("env endpoint set: spans should be recording (export mode)")
	}
}

func TestInit_NoEndpointAnywhere_NoExport(t *testing.T) {
	clearOTLPEnv(t)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "noexp"})
	defer obs.Close()

	if recording(t) {
		t.Error("no endpoint anywhere: spans must stay non-recording (zero-cost mode)")
	}
}

// Service is still required to export: an env endpoint without a service
// name must not push anonymous telemetry.
func TestInit_EnvEndpointWithoutService_NoExport(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+fakeCollector(t))
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{})
	defer obs.Close()

	if recording(t) {
		t.Error("env endpoint but no Service: must not export")
	}
}

// https:// → TLS and http:// / bare host:port → plaintext is unit-tested
// deterministically in internal/adapter/outbound/otel (TestResolveEndpoint);
// dialling a TLS endpoint from here would only buy a 5s retry wait.

// Init closes the previous default Obs. A second Close on an Obs the
// caller already closed must be a no-op, not another exporter flush.
func TestObs_Close_Idempotent(t *testing.T) {
	clearOTLPEnv(t)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "twice", OTel: "http://" + fakeCollector(t)})
	_ = recording(t)
	_ = obs.Close()

	start := time.Now()
	if err := obs.Close(); err != nil {
		t.Errorf("second Close error: %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("second Close took %v; want immediate no-op", d)
	}
}

package boeng_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// appTracerProvider stands in for a service that built its own SDK
// TracerProvider (own resource, own sampler, own exporter) and made it
// the process global before calling boeng.Init. The exporter is the
// in-memory one, so a test can prove where a span actually went. (An SDK
// provider with no span processor records nothing, so the exporter is
// also what makes the spans recording at all.)
func appTracerProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	res := resource.NewSchemaless(attribute.String("team", "shinonsen"))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSyncer(exp),
	)
	t.Cleanup(func() {
		// Leave no shut-down provider behind as the global for later tests.
		otel.SetTracerProvider(noop.NewTracerProvider())
		_ = tp.Shutdown(context.Background())
	})
	return tp, exp
}

// spanInRun reports, from inside the op (a span stops "recording" once it
// ends), whether boeng's span was recording and its span context.
func spanInRun(t *testing.T) (recording bool, sc trace.SpanContext) {
	t.Helper()
	_ = boeng.Run(context.Background(), "adopt_probe", nil, func(ctx context.Context) error {
		sp := trace.SpanFromContext(ctx)
		recording, sc = sp.IsRecording(), sp.SpanContext()
		return nil
	})
	return recording, sc
}

// No endpoint anywhere + a process TracerProvider already installed:
// boeng must open its spans on THAT provider (its resource, its sampler,
// its exporter) instead of replacing it with a private one.
func TestInit_AdoptsProcessTracerProvider(t *testing.T) {
	clearOTLPEnv(t)
	app, exported := appTracerProvider(t)
	otel.SetTracerProvider(app)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "adopt"})

	if rec, _ := spanInRun(t); !rec {
		t.Fatal("span should follow the app sampler (AlwaysSample) — boeng built its own provider instead")
	}
	var found bool
	for _, ss := range exported.GetSpans() {
		if ss.Name == "adopt_probe" {
			found = true
			if !ss.Resource.Set().HasValue("team") {
				t.Errorf("span resource lacks the app's attributes: %v", ss.Resource.Attributes())
			}
		}
	}
	if !found {
		t.Errorf("boeng span never reached the app's exporter; got %d spans", len(exported.GetSpans()))
	}
	if otel.GetTracerProvider() != trace.TracerProvider(app) {
		t.Error("Init replaced the process TracerProvider")
	}

	// Closing boeng must not shut down a provider it does not own.
	_ = obs.Close()
	_, after := app.Tracer("app").Start(context.Background(), "after-close")
	defer after.End()
	if !after.IsRecording() {
		t.Error("obs.Close shut down the app's TracerProvider")
	}
}

// Re-Init (hot reload, tests) must not "adopt" the provider boeng itself
// installed on the previous Init — that one is about to be shut down.
func TestInit_TwiceDoesNotAdoptItsOwnProvider(t *testing.T) {
	clearOTLPEnv(t)
	otel.SetTracerProvider(noop.NewTracerProvider())
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	first := boeng.Init(boeng.Config{Service: "twice"})
	second := boeng.Init(boeng.Config{Service: "twice"}) // closes first
	defer second.Close()
	_ = first

	if _, sc := spanInRun(t); !sc.IsValid() {
		t.Error("second Init produced spans without a valid context — it adopted the provider the first Init installed and then shut down")
	}
}

// Init never called, but the process has a TracerProvider: Run must use
// the global tracer rather than a no-op span that produces nothing.
func TestRun_WithoutInit_UsesGlobalTracer(t *testing.T) {
	clearOTLPEnv(t)
	app, exported := appTracerProvider(t)
	otel.SetTracerProvider(app)
	boeng.ResetDefaultForTest()

	if rec, _ := spanInRun(t); !rec {
		t.Error("Run without Init should open spans on the global TracerProvider")
	}
	if len(exported.GetSpans()) == 0 {
		t.Error("span from Run without Init never reached the process exporter")
	}
}

// The shinonsen shape: the service configured OpenTelemetry itself (SDK
// TracerProvider installed AND OTEL_EXPORTER_OTLP_ENDPOINT in the env for
// its own exporter). boeng must adopt that provider, not read the env as
// permission to build a second exporting pipeline over it.
func TestInit_AdoptsProcessTracerProviderEvenWithEnvEndpoint(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+fakeCollector(t))
	app, exported := appTracerProvider(t)
	otel.SetTracerProvider(app)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "adopt-env"})
	defer obs.Close()

	_, _ = spanInRun(t)
	if otel.GetTracerProvider() != trace.TracerProvider(app) {
		t.Fatal("env endpoint made Init replace the application's TracerProvider")
	}
	var found bool
	for _, ss := range exported.GetSpans() {
		found = found || ss.Name == "adopt_probe"
	}
	if !found {
		t.Error("boeng span did not reach the application's exporter")
	}
}

// An explicit Config.OTel is an instruction to export through boeng; it
// still wins over an installed provider.
func TestInit_ExplicitEndpointWinsOverProcessProvider(t *testing.T) {
	clearOTLPEnv(t)
	app, _ := appTracerProvider(t)
	otel.SetTracerProvider(app)
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "explicit", OTel: "http://" + fakeCollector(t)})
	defer obs.Close()

	if otel.GetTracerProvider() == trace.TracerProvider(app) {
		t.Error("explicit Config.OTel should make boeng install its own exporting provider")
	}
}

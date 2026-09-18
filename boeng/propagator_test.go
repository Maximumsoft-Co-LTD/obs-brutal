package boeng_test

import (
	"slices"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

func propagatorFields() []string { return otel.GetTextMapPropagator().Fields() }

// A process that already chose its propagator (TraceContext only, per a
// W3C-TraceContext-only policy) keeps it. boeng must not overwrite it
// with its TraceContext+Baggage composite.
func TestInit_RespectsExistingPropagator(t *testing.T) {
	clearOTLPEnv(t)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "prop"})
	defer obs.Close()

	got := propagatorFields()
	if slices.Contains(got, "baggage") {
		t.Errorf("Init overwrote the process propagator; fields now %v", got)
	}
	if !slices.Contains(got, "traceparent") {
		t.Errorf("traceparent missing from propagator fields %v", got)
	}
}

// With nothing configured (the global is the empty composite Go's OTel
// ships with) boeng installs TraceContext+Baggage so its adapters can
// propagate across processes.
func TestInit_InstallsDefaultPropagatorWhenUnset(t *testing.T) {
	clearOTLPEnv(t)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	obs := boeng.Init(boeng.Config{Service: "prop"})
	defer obs.Close()

	got := propagatorFields()
	for _, want := range []string{"traceparent", "tracestate", "baggage"} {
		if !slices.Contains(got, want) {
			t.Errorf("default propagator missing %q: %v", want, got)
		}
	}
}

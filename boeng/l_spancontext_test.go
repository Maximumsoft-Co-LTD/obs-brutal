package boeng_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// L(ctx) must recover trace correlation from ANY valid OTel span in ctx,
// not only from spans boeng opened itself. Third-party middleware (a
// hand-rolled Gin/otelgin layer, otelhttp, a gRPC interceptor) puts a raw
// span in ctx; log lines written under that ctx should still carry
// trace_id/span_id.
func TestL_FallsBackToRawSpanContext(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		ctx, span := otel.Tracer("third-party").Start(context.Background(), "raw")
		defer span.End()
		sc := span.SpanContext()
		if !sc.IsValid() {
			t.Fatal("test precondition: Init must install a tracer that yields valid span contexts")
		}

		boeng.L(ctx).Info("under raw span")

		e := findEntryWithMsg(t, sink.snapshot(), "under raw span")
		// The pipeline promotes trace_id/span_id out of F into the entry's
		// dedicated TraceID/SpanID slots (same as for boeng-opened spans).
		if e.TraceID != sc.TraceID().String() {
			t.Errorf("trace_id = %q, want %s", e.TraceID, sc.TraceID())
		}
		if e.SpanID != sc.SpanID().String() {
			t.Errorf("span_id = %q, want %s", e.SpanID, sc.SpanID())
		}
		// Root fields from Init must survive the fallback.
		if e.F["service"] != "g" {
			t.Errorf("service field lost in fallback: %v", e.F)
		}
	})
}

func TestL_NoSpanNoTraceFields(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		boeng.L(context.Background()).Info("plain")
		e := findEntryWithMsg(t, sink.snapshot(), "plain")
		if e.TraceID != "" {
			t.Errorf("no span in ctx must not fabricate trace_id: %q", e.TraceID)
		}
	})
}

// A boeng op nested under a raw third-party span keeps op-state priority:
// the op's own span_id (child), same trace_id.
func TestL_OpStateWinsOverRawSpan(t *testing.T) {
	runGuarantee(t, boeng.Config{Service: "g"}, func(t *testing.T, sink *captureSink) {
		ctx, span := otel.Tracer("third-party").Start(context.Background(), "raw")
		defer span.End()
		_ = boeng.Run(ctx, "nested_op", nil, func(ctx context.Context) error {
			boeng.L(ctx).Info("inside")
			return nil
		})
		e := findEntryWithMsg(t, sink.snapshot(), "inside")
		if e.TraceID != span.SpanContext().TraceID().String() {
			t.Errorf("trace_id should be inherited from raw parent: %q", e.TraceID)
		}
		if e.SpanID == span.SpanContext().SpanID().String() {
			t.Errorf("span_id should be the boeng child span, not the raw parent")
		}
		if e.F["op"] != "nested_op" {
			t.Errorf("op field missing: %v", e.F)
		}
	})
}

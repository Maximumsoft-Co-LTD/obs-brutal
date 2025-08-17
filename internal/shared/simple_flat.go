package shared

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// StartFlatSpan creates a sibling span (same trace, no parent-child relationship)
// ใช้สำหรับสร้าง span แบบ flat structure ใน trace เดียวกัน
func StartFlatSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Check if tracing is available
	if otel.GetTracerProvider() == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := otel.Tracer("obsvbrutal")
	if tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	// ถ้าไม่มี parent span ให้สร้าง root span ปกติ
	parentSpan := trace.SpanFromContext(ctx)
	if !parentSpan.SpanContext().IsValid() {
		return tracer.Start(ctx, name, opts...)
	}

	// สร้าง context ใหม่ที่มี trace ID เดียวกันแต่ไม่มี parent span
	// สร้าง SpanContext ใหม่ที่มี trace ID เดียวกันแต่ไม่มี parent
	newSpanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    parentSpan.SpanContext().TraceID(),
		SpanID:     trace.SpanID{}, // ไม่ระบุ parent span ID
		TraceFlags: parentSpan.SpanContext().TraceFlags(),
		TraceState: parentSpan.SpanContext().TraceState(),
		Remote:     true,
	})

	traceCtx := trace.ContextWithSpanContext(ctx, newSpanContext)

	// สร้าง span ใหม่ใน trace เดียวกันแต่ไม่เป็น child
	newCtx, span := tracer.Start(traceCtx, name, opts...)

	// Return context ที่มี span ใหม่
	return newCtx, span
}

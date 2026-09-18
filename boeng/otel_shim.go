package boeng

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	outboundotel "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/otel"
)

// otelProviderShim is a thin wrapper that lets boeng treat OTEL as optional.
// When provider == nil, all methods are no-ops returning a no-op span.
type otelProviderShim struct {
	provider *outboundotel.Provider
}

func newOTelShim(p *outboundotel.Provider) *otelProviderShim {
	if p == nil {
		return nil
	}
	return &otelProviderShim{provider: p}
}

func (s *otelProviderShim) Start(ctx context.Context, name string) (context.Context, trace.Span) {
	if s == nil || s.provider == nil {
		return ctx, noopSpan{}
	}
	return s.provider.StartSpan(ctx, name)
}

func (s *otelProviderShim) Shutdown(ctx context.Context) error {
	if s == nil || s.provider == nil {
		return nil
	}
	return s.provider.Shutdown(ctx)
}

// InstallMeterProvider makes the provider's MeterProvider the global one
// so boeng's per-op instruments record into the in-process registry —
// unless the application owns the global already (MeterForeign).
func (s *otelProviderShim) InstallMeterProvider() outboundotel.MeterInstall {
	if s == nil || s.provider == nil {
		return outboundotel.MeterForeign
	}
	return s.provider.InstallMeterProvider()
}

func (s *otelProviderShim) PrometheusHandler() http.Handler {
	return s.provider.PrometheusHandler()
}

// noopSpan is returned when OTEL is disabled, so callers don't need nil checks.
type noopSpan struct{ trace.Span }

func (noopSpan) End(...trace.SpanEndOption)              {}
func (noopSpan) SpanContext() trace.SpanContext          { return trace.SpanContext{} }
func (noopSpan) IsRecording() bool                       { return false }
func (noopSpan) SetStatus(codes.Code, string)            {}
func (noopSpan) SetName(string)                          {}
func (noopSpan) SetAttributes(...attribute.KeyValue)     {}
func (noopSpan) RecordError(error, ...trace.EventOption) {}
func (noopSpan) AddEvent(string, ...trace.EventOption)   {}
func (noopSpan) AddLink(trace.Link)                      {}
func (noopSpan) TracerProvider() trace.TracerProvider    { return nil }

package telemetry

import (
	"context"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// OTelTelemetry implements port.TelemetryProvider using OpenTelemetry SDK
type OTelTelemetry struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter
	logCounter     metric.Int64Counter
	errorCounter   metric.Int64Counter
	durationHist   metric.Float64Histogram
}

func NewOTelTelemetry(serviceName string, res *resource.Resource, tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider) (*OTelTelemetry, error) {
	if res == nil {
		res = resource.Default()
	}
	if tp == nil {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}
	if mp == nil {
		mp = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	}
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	t := &OTelTelemetry{tracerProvider: tp, meterProvider: mp, tracer: tp.Tracer(serviceName), meter: mp.Meter(serviceName)}
	if err := t.initMetrics(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *OTelTelemetry) initMetrics() error {
	var err error
	t.logCounter, err = t.meter.Int64Counter("logs_total")
	if err != nil {
		return err
	}
	t.errorCounter, err = t.meter.Int64Counter("errors_total")
	if err != nil {
		return err
	}
	t.durationHist, err = t.meter.Float64Histogram("log_duration_seconds")
	return err
}

// TelemetryProvider impl
func (t *OTelTelemetry) Tracer() port.Tracer         { return t }
func (t *OTelTelemetry) Meter() port.Meter           { return t }
func (t *OTelTelemetry) Propagator() port.Propagator { return t }
func (t *OTelTelemetry) ExtractTraceInfo(ctx context.Context) (string, string) {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return "", ""
}
func (t *OTelTelemetry) RecordLog(ctx context.Context, level domain.Level, d time.Duration, labels map[string]string) {
	attrs := make([]attribute.KeyValue, 0, len(labels)+1)
	attrs = append(attrs, attribute.String("level", level.String()))
	for k, v := range labels {
		attrs = append(attrs, attribute.String(k, v))
	}
	t.logCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if level >= domain.ErrorLevel {
		t.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	t.durationHist.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}
func (t *OTelTelemetry) Shutdown(ctx context.Context) error {
	if err := t.tracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	return t.meterProvider.Shutdown(ctx)
}

// Tracer impl
func (t *OTelTelemetry) StartSpan(ctx context.Context, name string, options ...any) (context.Context, any) {
	return t.tracer.Start(ctx, name)
}

// Meter impl
func (t *OTelTelemetry) IncCounter(ctx context.Context, name string, labels map[string]string) {
	c, _ := t.meter.Int64Counter(name)
	var attrs []attribute.KeyValue
	for k, v := range labels {
		attrs = append(attrs, attribute.String(k, v))
	}
	c.Add(ctx, 1, metric.WithAttributes(attrs...))
}
func (t *OTelTelemetry) ObserveHistogram(ctx context.Context, name string, value float64, labels map[string]string) {
	h, _ := t.meter.Float64Histogram(name)
	var attrs []attribute.KeyValue
	for k, v := range labels {
		attrs = append(attrs, attribute.String(k, v))
	}
	h.Record(ctx, value, metric.WithAttributes(attrs...))
}

// Propagator impl (simplified)
func (t *OTelTelemetry) Inject(ctx context.Context, carrier any)                  {}
func (t *OTelTelemetry) Extract(ctx context.Context, carrier any) context.Context { return ctx }

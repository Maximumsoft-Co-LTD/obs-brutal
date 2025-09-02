package otel

import (
    "obs-brutal/internal/core/domain"
    "obs-brutal/internal/core/port"
    service "obs-brutal/internal/core/service"
    telem "obs-brutal/internal/adapter/outbound/telemetry"
)

// Public adapter API for constructing providers and OTel-enabled loggers
type OTelLogBrt = service.OTelLogBrt

func NewOTelLogBrt(serviceName, version, environment, endpoint string, level domain.Level, sinks ...port.Sink) (*OTelLogBrt, error) {
    provider, err := NewOTelProvider(serviceName, version, environment, endpoint)
    if err != nil { return nil, err }
    tp, err := telem.NewOTelTelemetry(provider.ServiceName, nil, provider.TraceProvider, provider.MetricProvider)
    if err != nil { return nil, err }
    // Core no longer holds concrete provider; we build logger with TelemetryProvider only
    return service.NewOTelLogBrtWithProvider(tp, level, sinks...), nil
}

func NewOTelLogBrtWithProvider(tp port.TelemetryProvider, level domain.Level, sinks ...port.Sink) *OTelLogBrt {
    return service.NewOTelLogBrtWithProvider(tp, level, sinks...)
}

// NewOTel constructs both the OTel-enabled logger and the concrete Provider for integration needs (e.g., Prometheus handler).
func NewOTel(serviceName, version, environment, endpoint string, level domain.Level, sinks ...port.Sink) (*OTelLogBrt, *Provider, error) {
    provider, err := NewOTelProvider(serviceName, version, environment, endpoint)
    if err != nil { return nil, nil, err }
    tp, err := telem.NewOTelTelemetry(provider.ServiceName, nil, provider.TraceProvider, provider.MetricProvider)
    if err != nil { return nil, nil, err }
    log := service.NewOTelLogBrtWithProvider(tp, level, sinks...)
    return log, provider, nil
}

package shared

import (
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/otel"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	service "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/service"
)

// Factory centralizes creation of sinks and wiring into core loggers.
type Factory struct{}

func New() *Factory { return &Factory{} }

// NewLogbrut wires sinks (adapters) into core logger based on provided sinks.
func NewLogbrut(level domain.Level, sinks ...port.Sink) service.LogBrt {
	return service.NewUnifiedLogBrt(level, sinks...)
}

// NewOTelLogbrut constructs OTel-enabled logger (core) with provided sinks.
func NewOTelLogbrut(serviceName, version, environment, endpoint string, level domain.Level, sinks ...port.Sink) (*otel.OTelLogBrt, error) {
	return otel.NewOTelLogBrt(serviceName, version, environment, endpoint, level, sinks...)
}

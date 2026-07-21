package port

import (
	"context"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"time"
)

// Tracer is an abstraction for tracing
type Tracer interface {
	StartSpan(ctx context.Context, name string, options ...any) (context.Context, any)
}

// Meter is an abstraction for metrics
type Meter interface {
	IncCounter(ctx context.Context, name string, labels map[string]string)
	ObserveHistogram(ctx context.Context, name string, value float64, labels map[string]string)
}

// Propagator abstracts context propagation across boundaries
type Propagator interface {
	Inject(ctx context.Context, carrier any)
	Extract(ctx context.Context, carrier any) context.Context
}

// TelemetryProvider groups tracer/meter/propagator
type TelemetryProvider interface {
	Tracer() Tracer
	Meter() Meter
	Propagator() Propagator
	ExtractTraceInfo(ctx context.Context) (traceID, spanID string)
	RecordLog(ctx context.Context, level domain.Level, duration time.Duration, labels map[string]string)
	Shutdown(ctx context.Context) error
}

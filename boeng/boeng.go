package boeng

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"obs-brutal/internal/core/service/base"
	"obs-brutal/internal/logtrc"
)

// Logger is the operation-scoped fluent logger boeng exposes through
// L(ctx) and through adapter packages (boeng/gin, boeng/http, etc.).
// Most application code never references this type directly; it just
// receives one through the adapters.
type Logger = logtrc.LogBrt

// Config configures the global Obs handle. Pass to Init once at process start.
type Config struct {
	Service string       // service name (required for OTEL)
	Version string       // service version, e.g. "1.0.0"
	Env     string       // dev | uat | prod
	OTel    string       // OTLP gRPC endpoint, "" disables OTEL
	Loki    string       // Loki push URL, "" disables Loki sink
	Async bool  // use async pipeline
	Level Level // log level; zero = INFO

	// IncludeZeroFields, when true, makes the reflection fallback emit
	// zero-valued exported fields. Default false — zeros are skipped to
	// keep log lines lean.
	IncludeZeroFields bool

	// MetricLabels is the allowlist of subject field keys that may be
	// promoted to per-op metric labels. "service" and "env" are always
	// included automatically. Only add LOW-CARDINALITY keys here (typical
	// rule of thumb: fewer than 100 distinct values). Adding a high-
	// cardinality key like user_id or order_id explodes the Prometheus
	// time-series cardinality and can crash the metrics backend.
	//
	// Examples of safe values: "user_type", "payment_channel", "tier",
	// "region". Examples to keep OUT: any unique entity id, free-form
	// text, IPs.
	MetricLabels []string
}

// Obs holds the configured logger and (optional) OTEL provider.
type Obs struct {
	cfg      Config
	log      logtrc.LogBrt
	provider *otelProviderShim // nil when OTEL disabled
}

var (
	mu  sync.RWMutex
	def *Obs
)

// Init builds an Obs from Config, stashes it as the package default
// (so Run/Enter/L can find it without explicit passing), and returns it.
// Calling Init twice replaces the default and closes the previous one
// (best-effort: async pipeline flushed, OTel exporter shut down).
func Init(cfg Config) *Obs {
	// Always install a W3C TraceContext + Baggage propagator so the
	// adapter packages (boeng/gin, boeng/http, boeng/rabbit, ...) can
	// inject and extract trace context across process boundaries even
	// when no OTLP exporter is configured. Without this, distributed
	// trace lineage breaks the moment a request crosses a service.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	level := cfg.Level
	sinks := buildSinks(cfg)

	setFieldsConfig(FieldsConfig{IncludeZeroFields: cfg.IncludeZeroFields})
	configureMetrics(cfg)

	o := &Obs{cfg: cfg}

	if cfg.OTel != "" && cfg.Service != "" {
		ot, prov, err := logtrc.NewOTelWithService(
			cfg.Service, cfg.Version, cfg.Env, cfg.OTel, level, sinks...,
		)
		if err == nil && ot != nil {
			o.log = ot
			o.provider = newOTelShim(prov)
		}
	}
	if o.log == nil {
		if cfg.Async {
			o.log = logtrc.NewAsyncLogBrt(level, sinks...)
		} else {
			// Sync path: build the unified logger directly with the
			// configured sinks so callers (and tests) can inject their
			// own. logtrc.NewDefault would bypass sinks completely.
			o.log = base.NewUnifiedLogBrt(level, sinks...)
		}
	}
	if cfg.Service != "" {
		o.log = o.log.F("service", cfg.Service)
	}
	if cfg.Env != "" {
		o.log = o.log.F("env", cfg.Env)
	}

	mu.Lock()
	prev := def
	def = o
	mu.Unlock()
	// Best-effort cleanup of the previous default so repeat Init calls
	// (tests, hot-reload, config flips) don't leak the old async pipeline
	// or OTel exporter goroutines. Errors are intentionally ignored — a
	// failure here must not prevent the new Obs from going live.
	if prev != nil && prev != o {
		_ = prev.Close()
	}
	return o
}

// Close releases resources (async flush, OTEL shutdown). Safe to call on nil.
func (o *Obs) Close() error {
	if o == nil {
		return nil
	}
	type stopper interface{ Stop() }
	if s, ok := o.log.(stopper); ok {
		s.Stop()
	}
	if o.provider != nil {
		return o.provider.Shutdown(context.Background())
	}
	return nil
}

// D returns the package-default Obs (or nil if Init was never called).
func D() *Obs {
	mu.RLock()
	defer mu.RUnlock()
	return def
}

// Log returns the root logger of this Obs. Most callers want boeng.L(ctx)
// instead, which returns the request/operation-scoped logger.
func (o *Obs) Log() logtrc.LogBrt {
	if o == nil {
		return logtrc.NewDefault()
	}
	return o.log
}

// testSinkOverride, when non-nil, replaces all sinks Init would otherwise
// build from Config. Set via export_test.go SetSinkForTest. Production
// code never touches it.
var testSinkOverride logtrc.Sink

func buildSinks(cfg Config) []logtrc.Sink {
	if testSinkOverride != nil {
		return []logtrc.Sink{testSinkOverride}
	}
	sinks := []logtrc.Sink{logtrc.NewFastStdoutSink()}
	if cfg.Loki != "" {
		labels := map[string]string{
			"app": cfg.Service,
			"env": cfg.Env,
		}
		sinks = append(sinks, logtrc.NewLokiPushSink(cfg.Loki, labels))
	}
	return sinks
}

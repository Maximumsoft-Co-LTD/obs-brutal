package boeng

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	outboundotel "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/otel"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/service/base"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/logtrc"
)

// Logger is the operation-scoped fluent logger boeng exposes through
// L(ctx) and through adapter packages (boeng/gin, boeng/http, etc.).
// Most application code never references this type directly; it just
// receives one through the adapters.
type Logger = logtrc.LogBrt

// Config configures the global Obs handle. Pass to Init once at process start.
type Config struct {
	Service string // service name (required for OTEL)
	Version string // service version, e.g. "1.0.0"
	Env     string // dev | uat | prod
	// OTel is the OTLP gRPC endpoint as a URL, per the OTel exporter
	// spec: "http://collector:4317" (plaintext) or
	// "https://collector:4317" (TLS, system roots). Bare "host:4317" is
	// still accepted and means plaintext. Empty → boeng exports only if
	// OTEL_EXPORTER_OTLP_ENDPOINT (or the _TRACES_/_METRICS_ variant) is
	// set in the environment, letting the SDK read endpoint, headers,
	// timeout and certificates itself. Nothing set → no export.
	OTel  string
	Loki  string // Loki push URL, "" disables Loki sink
	Async bool   // use async pipeline
	Level Level  // minimum level written; zero = DebugLevel (nothing filtered)

	// QuietOps demotes the success-path "<op> completed" line from INFO
	// to DEBUG. Failures ("<op> failed") stay at ERROR. Use it in
	// production to run at Level=InfoLevel — so Emit events and manual
	// op.Log lines remain visible — without paying for one INFO line per
	// operation per request. The "<op> started" line is DEBUG already.
	QuietOps bool

	// EmitLevel is the level Emit / op.Emit write their event line at.
	// Zero means INFO. Set it to WarnLevel when a deployment runs at
	// Level=WarnLevel but still needs its per-request summary events
	// (dashboards fed from log-derived metrics go dark otherwise).
	// Emit is never written at DEBUG: an event not worth INFO should not
	// be an event.
	EmitLevel Level

	// IncludeZeroFields, when true, makes the reflection fallback emit
	// zero-valued exported fields. Default false — zeros are skipped to
	// keep log lines lean.
	IncludeZeroFields bool

	// MetricSchema selects how per-operation metrics are named. The zero
	// value PerOpMetrics keeps the v1.x shape (<op>_total, <op>_duration_ms,
	// ...). LabeledMetrics emits one fixed family with op/outcome labels
	// (boeng_operation_duration_seconds, boeng_events_total) so
	// `sum by (op)` works in PromQL. BothMetrics emits both during a
	// dashboard migration. See "Metrics" in the package README.
	MetricSchema MetricSchema

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
	closed   atomic.Bool
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
	// Make sure a propagator exists so the adapter packages (boeng/gin,
	// boeng/http, boeng/rabbit, ...) can carry trace context across
	// process boundaries even with no OTLP exporter. A propagator the
	// process already chose (e.g. TraceContext only) is kept as is.
	outboundotel.EnsurePropagator()

	level := cfg.Level
	sinks := buildSinks(cfg)

	setFieldsConfig(FieldsConfig{IncludeZeroFields: cfg.IncludeZeroFields})
	configureMetrics(cfg)
	configureLevels(cfg)

	o := &Obs{cfg: cfg}

	// Where spans go, in priority order:
	//  1. Config.OTel set → boeng builds and installs its own exporting
	//     provider: the caller asked boeng to export.
	//  2. The application already installed an SDK TracerProvider → adopt
	//     it (its resource, sampler, exporter); boeng adds operations to
	//     the process's trace pipeline instead of running a second one.
	//     An OTEL_EXPORTER_OTLP_ENDPOINT in the env belongs to that
	//     pipeline, so it must not override this step.
	//  3. OTEL_EXPORTER_OTLP_*ENDPOINT in the env → boeng builds its own
	//     exporting provider and lets the SDK read the env (endpoint,
	//     headers, timeout, certs), the OTel-spec way to configure export.
	//  4. Nothing → a non-exporting provider so spans still carry valid
	//     W3C contexts (trace_id in logs, traceparent propagation).
	// A service name is required to export (1, 3): no anonymous telemetry.
	adoptTP, adoptable := outboundotel.AdoptableGlobalTracerProvider()
	exportViaBoeng := cfg.Service != "" && (cfg.OTel != "" || (!adoptable && outboundotel.ExportConfigured("")))
	if exportViaBoeng {
		ot, prov, err := logtrc.NewOTelWithService(
			cfg.Service, cfg.Version, cfg.Env, cfg.OTel, level, sinks...,
		)
		if err == nil && ot != nil {
			o.log = ot
			o.provider = newOTelShim(prov)
		}
	}
	if o.provider == nil {
		var prov *outboundotel.Provider
		var err error
		if adoptable {
			prov, err = outboundotel.NewOTelProviderAdopting(cfg.Service, cfg.Version, cfg.Env, adoptTP)
		} else {
			prov, err = outboundotel.NewOTelProviderNoExport(cfg.Service, cfg.Version, cfg.Env)
		}
		if err == nil {
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

// Close releases resources (async flush, OTEL shutdown). Safe to call on
// nil and idempotent: only the first call does work. That matters because
// Init closes the previous default; an Obs the caller already closed must
// not pay the exporter flush deadline a second time.
func (o *Obs) Close() error {
	if o == nil {
		return nil
	}
	if !o.closed.CompareAndSwap(false, true) {
		return nil
	}
	type stopper interface{ Stop() }
	if s, ok := o.log.(stopper); ok {
		s.Stop()
	}
	if o.provider != nil {
		// Bound shutdown: TracerProvider/MeterProvider.Shutdown flushes
		// pending spans/metrics through the OTLP exporter, which retries
		// on a dead collector. With context.Background() a shutdown while
		// the collector is unreachable blocked the process on exit for
		// the exporter's full retry window (~1 min). A deadline caps that.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return o.provider.Shutdown(ctx)
	}
	return nil
}

// MetricsHandler returns an http.Handler that serves this Obs's
// per-operation metrics (<op>_total, <op>_duration_ms, <op>_error_total,
// <op>_panic_total, <event>_total) in Prometheus exposition format from
// the in-process registry. Mount it on the application's own HTTP server:
//
//	mux.Handle("/metrics", obs.MetricsHandler())
//
// It is the pull-based fallback for deployments whose OTLP collector does
// not accept metrics. When Config.OTel is empty, boeng records no metrics
// at all by default (zero-cost mode); the first call to MetricsHandler
// switches metric recording on so the page is never silently empty.
//
// If the application installed its own SDK MeterProvider, boeng never
// replaces it: the per-op instruments record into the application's
// pipeline instead, and this handler answers 503 with a line saying so.
// A failing scrape (up=0) is the honest signal here — the metrics exist,
// but they leave through the application's exporter, not this endpoint.
// Safe to call on a nil *Obs — the handler then serves an empty page.
func (o *Obs) MetricsHandler() http.Handler {
	if o == nil || o.provider == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		})
	}
	switch o.provider.InstallMeterProvider() {
	case outboundotel.MeterInstalled:
		// Instruments minted before the install are bound to the no-op
		// global meter; drop them so ops rebind on next use.
		resetMetricInstruments()
	case outboundotel.MeterForeign:
		o.log.Warn("boeng.MetricsHandler: the application owns the global MeterProvider; boeng metrics record into it and are not served here")
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("boeng metrics record into the application's MeterProvider and are exported through it; this endpoint has nothing to serve\n"))
		})
	}
	return o.provider.PrometheusHandler()
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

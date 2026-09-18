package otel

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	promcli "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider wraps OTEL tracer/meter providers and metrics helpers
type Provider struct {
	ServiceName string
	Version     string
	Environment string

	TraceProvider  *sdktrace.TracerProvider
	MetricProvider *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter

	logCounter   metric.Int64Counter
	errorCounter metric.Int64Counter
	durationHist metric.Float64Histogram

	promRegistry *promcli.Registry

	meterInstalled atomic.Bool
}

// ownedTracerProvider is the TracerProvider boeng itself installed as the
// process global (if any). AdoptableGlobalTracerProvider uses it to tell
// "the application's provider" apart from "our own previous Init", which
// is about to be shut down and must never be adopted.
var ownedTracerProvider atomic.Pointer[sdktrace.TracerProvider]

// ownedMeterProvider is the MeterProvider boeng itself installed as the
// process global (if any), so InstallMeterProvider can tell the
// application's MeterProvider apart from boeng's own previous Init.
var ownedMeterProvider atomic.Pointer[sdkmetric.MeterProvider]

// MeterInstall is the outcome of InstallMeterProvider.
type MeterInstall int

const (
	// MeterInstalled: boeng's MeterProvider became the global on this call.
	MeterInstalled MeterInstall = iota
	// MeterAlreadyInstalled: boeng's MeterProvider was the global already.
	MeterAlreadyInstalled
	// MeterForeign: the application installed its own SDK MeterProvider;
	// boeng left it in place and its instruments record into it.
	MeterForeign
)

// ForeignMeterProviderInstalled reports whether the global MeterProvider
// is an SDK provider the application (not boeng) installed.
func ForeignMeterProviderInstalled() bool {
	sdk, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	return ok && sdk != nil && sdk != ownedMeterProvider.Load()
}

// EnsurePropagator installs the W3C TraceContext + Baggage composite only
// when the process has not chosen a propagator yet — Go's OTel default is
// an empty composite whose Fields() is nil. A service that already set,
// say, TraceContext alone keeps it; boeng's adapters read the global at
// call time so they follow whatever is installed.
func EnsurePropagator() {
	if len(otel.GetTextMapPropagator().Fields()) > 0 {
		return
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}

// AdoptableGlobalTracerProvider reports the process's own SDK
// TracerProvider when the application installed one before boeng.Init —
// its resource, sampler and exporter are the ones boeng should open spans
// on. It returns false for the OTel no-op/default global and for a
// provider boeng installed itself.
func AdoptableGlobalTracerProvider() (trace.TracerProvider, bool) {
	tp := otel.GetTracerProvider()
	sdk, ok := tp.(*sdktrace.TracerProvider)
	if !ok || sdk == nil || sdk == ownedTracerProvider.Load() {
		return nil, false
	}
	return tp, true
}

// NewOTelProvider builds the tracer + meter provider.
//
// Export is decided by ExportConfigured: an explicit endpoint, or — when
// endpoint is empty — the OTEL_EXPORTER_OTLP_*ENDPOINT environment. In
// the env case no endpoint option is passed to the exporters, so the SDK
// reads the whole spec'd env set itself (endpoint, headers, timeout,
// certificates). With neither, the providers are still built (tracer
// with valid W3C contexts, in-process prometheus meter) but nothing is
// exported — the "no OTLP exporter" mode boeng relies on for trace_id in
// logs and traceparent propagation without a collector.
func NewOTelProvider(serviceName, version, environment, endpoint string) (*Provider, error) {
	return newProvider(serviceName, version, environment, endpoint, ExportConfigured(endpoint), nil)
}

// NewOTelProviderNoExport builds the no-collector provider regardless of
// the environment. boeng uses it as the fallback when export is not
// allowed (no service name) or the exporting constructor failed.
func NewOTelProviderNoExport(serviceName, version, environment string) (*Provider, error) {
	return newProvider(serviceName, version, environment, "", false, nil)
}

// NewOTelProviderAdopting builds a Provider whose spans come from tp — the
// application's TracerProvider — instead of a private one. boeng does not
// install, sample, export or shut down tp; it only opens spans on it. The
// meter side is still boeng's own (in-process Prometheus registry) so
// MetricsHandler keeps working.
func NewOTelProviderAdopting(serviceName, version, environment string, tp trace.TracerProvider) (*Provider, error) {
	return newProvider(serviceName, version, environment, "", false, tp)
}

// otlpEnvKeys are the spec'd endpoint variables the SDK honours; any one
// of them set means the deployment asked for export.
var otlpEnvKeys = []string{
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
}

// ExportConfigured reports whether telemetry should leave the process:
// an explicit endpoint wins; otherwise the OTLP endpoint environment
// variables decide, mirroring how every OTel SDK resolves its exporter.
func ExportConfigured(endpoint string) bool {
	if endpoint != "" {
		return true
	}
	for _, k := range otlpEnvKeys {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

// resolveEndpoint turns the configured endpoint into the gRPC target and
// the transport decision, following the OTel exporter spec: the URL
// scheme carries TLS intent. https:// → TLS with system roots; http://
// → plaintext; a bare host:port (pre-1.3 form) → plaintext, unchanged
// from what boeng always did. Any path component is dropped — gRPC
// targets are host:port.
func resolveEndpoint(endpoint string) (host string, insecure bool) {
	switch {
	case endpoint == "":
		return "", true
	case strings.HasPrefix(endpoint, "https://"):
		host, insecure = strings.TrimPrefix(endpoint, "https://"), false
	case strings.HasPrefix(endpoint, "http://"):
		host, insecure = strings.TrimPrefix(endpoint, "http://"), true
	default:
		host, insecure = endpoint, true
	}
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	return host, insecure
}

func newProvider(serviceName, version, environment, endpoint string, exporting bool, adopt trace.TracerProvider) (*Provider, error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes("", semconv.ServiceName(serviceName), semconv.ServiceVersion(version), semconv.DeploymentEnvironment(environment)))
	if err != nil {
		return nil, err
	}

	// When exporting, record fully (AlwaysSample). When not exporting,
	// use NeverSample: the tracer still generates valid W3C span contexts
	// — so trace_id/span_id reach the logs and traceparent propagates —
	// but the spans are non-recording, avoiding the cost of recording
	// attributes nothing will ever export.
	sampler := sdktrace.NeverSample()
	if exporting {
		sampler = sdktrace.AlwaysSample()
	}
	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res), sdktrace.WithSampler(sampler)}
	promReg := promcli.NewRegistry()
	promExporter, err := prometheus.New(prometheus.WithRegisterer(promReg))
	if err != nil {
		return nil, err
	}
	metricOpts := []sdkmetric.Option{sdkmetric.WithReader(promExporter), sdkmetric.WithResource(res)}

	if exporting {
		// Explicit endpoint → pass it as an option (options override the
		// SDK's env read). Empty endpoint → pass nothing and let the SDK
		// resolve OTEL_EXPORTER_OTLP_* itself, scheme-derived TLS included.
		var traceExpOpts []otlptracegrpc.Option
		var metricExpOpts []otlpmetricgrpc.Option
		if host, insecure := resolveEndpoint(endpoint); host != "" {
			traceExpOpts = append(traceExpOpts, otlptracegrpc.WithEndpoint(host))
			metricExpOpts = append(metricExpOpts, otlpmetricgrpc.WithEndpoint(host))
			if insecure {
				traceExpOpts = append(traceExpOpts, otlptracegrpc.WithInsecure())
				metricExpOpts = append(metricExpOpts, otlpmetricgrpc.WithInsecure())
			}
		}
		traceExporter, err := otlptracegrpc.New(context.Background(), traceExpOpts...)
		if err != nil {
			return nil, err
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(traceExporter))
		// Metrics leave the process via OTLP the same way traces do; the
		// prometheus reader above only feeds the in-process registry.
		metricExporter, err := otlpmetricgrpc.New(context.Background(), metricExpOpts...)
		if err != nil {
			return nil, err
		}
		metricOpts = append(metricOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second))))
	}

	mp := sdkmetric.NewMeterProvider(metricOpts...)
	p := &Provider{ServiceName: serviceName, Version: version, Environment: environment, MetricProvider: mp, meter: mp.Meter(serviceName), promRegistry: promReg}

	if adopt != nil {
		// The application owns the TracerProvider: open spans on it, touch
		// nothing global, and never shut it down.
		p.tracer = adopt.Tracer(serviceName)
	} else {
		// Install our tracer so span contexts and W3C propagation work
		// regardless of export, and remember it as ours so a later Init
		// does not mistake it for an application-owned provider.
		tp := sdktrace.NewTracerProvider(traceOpts...)
		otel.SetTracerProvider(tp)
		ownedTracerProvider.Store(tp)
		p.TraceProvider = tp
		p.tracer = tp.Tracer(serviceName)
	}
	// Respect a propagator the process already chose; install the default
	// composite only when none is set.
	EnsurePropagator()
	// Only take over the global meter provider when actually exporting —
	// otherwise leave metric recording as a no-op (boeng's per-op metrics
	// read the global meter) so a no-collector deployment keeps its zero-
	// metric cost. MetricsHandler installs it on demand.
	if exporting {
		p.InstallMeterProvider()
	}
	if err := p.initMetrics(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Provider) initMetrics() error {
	var err error
	p.logCounter, err = p.meter.Int64Counter("logs_total", metric.WithDescription("Total number of logs written"))
	if err != nil {
		return err
	}
	p.errorCounter, err = p.meter.Int64Counter("errors_total", metric.WithDescription("Total number of error logs"))
	if err != nil {
		return err
	}
	p.durationHist, err = p.meter.Float64Histogram("log_duration_seconds", metric.WithDescription("Time spent writing logs"), metric.WithUnit("s"))
	return err
}

func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}
func (p *Provider) ExtractTraceInfo(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return
}
func (p *Provider) RecordLogMetric(ctx context.Context, d time.Duration, level string, attrs ...attribute.KeyValue) {
	if ctx == nil {
		ctx = context.Background()
	}
	p.logCounter.Add(ctx, 1, metric.WithAttributes(append(attrs, attribute.String("level", level))...))
	if level == "ERROR" || level == "FATAL" {
		p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	p.durationHist.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}
func (p *Provider) InjectHeaders(ctx context.Context, headers http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}
func (p *Provider) ExtractHeaders(ctx context.Context, headers http.Header) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}

// InstallMeterProvider makes p.MetricProvider the global OTel meter
// provider unless the application already installed its own — boeng
// never replaces an application-owned pipeline; its instruments simply
// record into it. In exporting mode NewOTelProvider calls this itself; in
// the no-collector mode it is deferred until something (a /metrics
// handler) wants the metrics, so the zero-cost default holds.
func (p *Provider) InstallMeterProvider() MeterInstall {
	if p == nil || p.MetricProvider == nil {
		return MeterForeign
	}
	if ForeignMeterProviderInstalled() {
		return MeterForeign
	}
	if !p.meterInstalled.CompareAndSwap(false, true) {
		return MeterAlreadyInstalled
	}
	otel.SetMeterProvider(p.MetricProvider)
	ownedMeterProvider.Store(p.MetricProvider)
	return MeterInstalled
}

func (p *Provider) PrometheusHandler() http.Handler {
	if p.promRegistry == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(p.promRegistry, promhttp.HandlerOpts{})
}

// Shutdown flushes and stops what boeng owns. An adopted (application)
// TracerProvider is left untouched.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.TraceProvider != nil {
		if err := p.TraceProvider.Shutdown(ctx); err != nil {
			return err
		}
	}
	return p.MetricProvider.Shutdown(ctx)
}

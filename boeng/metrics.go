package boeng

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// defaultMetricLabels is the minimal allowlist used when Config.MetricLabels
// is left empty. Both are bounded by operational reality (service count and
// environment count are small) so they're safe defaults.
var defaultMetricLabels = []string{"service", "env"}

// maxDistinctMetricNames caps how many distinct sanitized op/event names
// may each mint their own metric series. It is the name-cardinality half
// of G3: the label allowlist bounds label cardinality, and this bounds
// metric-name cardinality.
//
// Op names flow from callers — and adapters build them from request data
// (an HTTP path with an embedded id, a RabbitMQ routing key with an
// order id). Without a cap, one such call site mints a new metric series
// per unique value, which both leaks the cache map without bound and can
// OOM the downstream Prometheus/Mimir. Past the cap, further names route
// to a shared "overflow" series (fail-closed) whose own counter is the
// operator's signal that a high-cardinality name is in play.
const maxDistinctMetricNames = 512

// opMetricSet holds the per-operation OTel instruments. They are created
// lazily the first time an op fires and cached for the lifetime of the
// process, because OTel SDK instruments are designed to be re-used.
type opMetricSet struct {
	total    metric.Int64Counter
	duration metric.Float64Histogram
	errors   metric.Int64Counter
	panics   metric.Int64Counter
}

// eventMetricSet is the cheaper sibling used for Emit — only a counter.
type eventMetricSet struct {
	total metric.Int64Counter
}

var (
	metricsMu       sync.RWMutex
	opMetricsCache  = map[string]*opMetricSet{}
	evtMetricsCache = map[string]*eventMetricSet{}

	labelsMu      sync.RWMutex
	labelAllowed  = toSet(defaultMetricLabels)
	staticAttrs   []attribute.KeyValue
	staticLabelKV = map[string]attribute.KeyValue{}

	// Shared fail-closed sinks used once the distinct-name cap is hit.
	opOverflow  *opMetricSet
	evtOverflow *eventMetricSet
)

func toSet(keys []string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		out[k] = struct{}{}
	}
	return out
}

// configureMetrics is called from Init. It refreshes the label allowlist and
// the static service/env labels that every metric carries.
func configureMetrics(cfg Config) {
	labelsMu.Lock()
	defer labelsMu.Unlock()

	allowed := append([]string(nil), defaultMetricLabels...)
	allowed = append(allowed, cfg.MetricLabels...)
	labelAllowed = toSet(allowed)

	staticAttrs = staticAttrs[:0]
	staticLabelKV = map[string]attribute.KeyValue{}
	if cfg.Service != "" {
		kv := attribute.String("service", cfg.Service)
		staticAttrs = append(staticAttrs, kv)
		staticLabelKV["service"] = kv
	}
	if cfg.Env != "" {
		kv := attribute.String("env", cfg.Env)
		staticAttrs = append(staticAttrs, kv)
		staticLabelKV["env"] = kv
	}
}

func opMetricsFor(name string) *opMetricSet {
	safe := sanitizeMetricName(name)
	metricsMu.RLock()
	if m, ok := opMetricsCache[safe]; ok {
		metricsMu.RUnlock()
		return m
	}
	metricsMu.RUnlock()

	metricsMu.Lock()
	defer metricsMu.Unlock()
	if m, ok := opMetricsCache[safe]; ok {
		return m
	}
	// Fail closed once the distinct-name budget is spent: route to a
	// shared overflow series instead of minting (and caching) another.
	if len(opMetricsCache) >= maxDistinctMetricNames {
		if opOverflow == nil {
			opOverflow = newOpMetricSet("overflow")
		}
		return opOverflow
	}
	m := newOpMetricSet(safe)
	opMetricsCache[safe] = m
	return m
}

func newOpMetricSet(safe string) *opMetricSet {
	meter := otel.GetMeterProvider().Meter("boeng")
	s := &opMetricSet{}
	var err error
	if s.total, err = meter.Int64Counter(safe + "_total"); err != nil {
		return &opMetricSet{}
	}
	if s.duration, err = meter.Float64Histogram(safe+"_duration_ms", metric.WithUnit("ms")); err != nil {
		return &opMetricSet{}
	}
	if s.errors, err = meter.Int64Counter(safe + "_error_total"); err != nil {
		return &opMetricSet{}
	}
	if s.panics, err = meter.Int64Counter(safe + "_panic_total"); err != nil {
		return &opMetricSet{}
	}
	return s
}

func eventMetricsFor(name string) *eventMetricSet {
	safe := sanitizeMetricName(name)
	metricsMu.RLock()
	if m, ok := evtMetricsCache[safe]; ok {
		metricsMu.RUnlock()
		return m
	}
	metricsMu.RUnlock()

	metricsMu.Lock()
	defer metricsMu.Unlock()
	if m, ok := evtMetricsCache[safe]; ok {
		return m
	}
	if len(evtMetricsCache) >= maxDistinctMetricNames {
		if evtOverflow == nil {
			evtOverflow = newEventMetricSet("overflow")
		}
		return evtOverflow
	}
	s := newEventMetricSet(safe)
	evtMetricsCache[safe] = s
	return s
}

func newEventMetricSet(safe string) *eventMetricSet {
	meter := otel.GetMeterProvider().Meter("boeng")
	c, err := meter.Int64Counter(safe + "_total")
	s := &eventMetricSet{}
	if err == nil {
		s.total = c
	}
	return s
}

// metricLabels turns subject fields into a flat slice of attribute.KeyValue
// suitable for OTel calls, filtered to the configured allowlist. Service/env
// from Init are always included; user-supplied keys must match the allowlist
// to escape the cardinality guard.
func metricLabels(subject any) []attribute.KeyValue {
	labelsMu.RLock()
	allowed := labelAllowed
	statics := staticAttrs
	labelsMu.RUnlock()

	fields := extractFields(subject)
	out := make([]attribute.KeyValue, 0, len(statics)+len(fields))
	out = append(out, statics...)
	if len(allowed) == 0 || len(fields) == 0 {
		return out
	}
	for k, v := range fields {
		if _, ok := allowed[k]; !ok {
			continue
		}
		if _, isStatic := staticLabelKV[k]; isStatic {
			continue
		}
		out = append(out, attribute.String(k, fmt.Sprint(v)))
	}
	return out
}

// recordOp emits the per-operation metrics: increments <op>_total, optionally
// <op>_error_total and <op>_panic_total, and records duration into the histogram.
// panicked implies errored. labels are reused across all four metrics.
func recordOp(ctx context.Context, name string, dur time.Duration, errored, panicked bool, labels []attribute.KeyValue) {
	m := opMetricsFor(name)
	opts := []metric.AddOption{metric.WithAttributes(labels...)}
	histOpts := []metric.RecordOption{metric.WithAttributes(labels...)}
	if m.total != nil {
		m.total.Add(ctx, 1, opts...)
	}
	if m.duration != nil {
		m.duration.Record(ctx, float64(dur.Milliseconds()), histOpts...)
	}
	if errored && m.errors != nil {
		m.errors.Add(ctx, 1, opts...)
	}
	if panicked && m.panics != nil {
		m.panics.Add(ctx, 1, opts...)
	}
}

// recordEvent bumps the per-event counter (cheap; events don't have duration).
func recordEvent(ctx context.Context, name string, labels []attribute.KeyValue) {
	m := eventMetricsFor(name)
	if m.total != nil {
		m.total.Add(ctx, 1, metric.WithAttributes(labels...))
	}
}

// sanitizeMetricName turns an arbitrary op or event name into a Prometheus/OTel
// safe metric name: lowercase letters, digits, and underscores only. Runs of
// disallowed characters collapse to a single underscore. Leading/trailing
// underscores are stripped. Empty input returns "unnamed".
//
//	"create_user"    → "create_user"
//	"GET /users/:id" → "get_users_id"
//	"  weird!!name " → "weird_name"
func sanitizeMetricName(s string) string {
	if s == "" {
		return "unnamed"
	}
	var b strings.Builder
	b.Grow(len(s))
	prevUnderscore := true // suppress leading underscore
	for _, r := range s {
		switch {
		case r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z'):
			b.WriteRune(r)
			prevUnderscore = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		default:
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "_")
	if out == "" {
		return "unnamed"
	}
	return out
}

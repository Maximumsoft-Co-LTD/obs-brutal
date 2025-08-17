package outbound

import (
	"context"
	"fmt"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/outbound"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// PrometheusMetricsProvider provides metrics using Prometheus
type PrometheusMetricsProvider struct {
	meter         metric.Meter
	logCounter    metric.Int64Counter
	errorCounter  metric.Int64Counter
	latencyHist   metric.Float64Histogram
	customMetrics map[string]interface{}
	mu            sync.RWMutex
}

// NewPrometheusMetricsProvider creates a new Prometheus metrics provider
func NewPrometheusMetricsProvider(serviceName string) (outbound.Metrics, error) {
	meter := otel.Meter(serviceName)

	// Create log counter
	logCounter, err := meter.Int64Counter(
		"log_messages_total",
		metric.WithDescription("Total number of log messages"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create log counter: %w", err)
	}

	// Create error counter
	errorCounter, err := meter.Int64Counter(
		"log_errors_total",
		metric.WithDescription("Total number of errors logged"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	// Create latency histogram
	latencyHist, err := meter.Float64Histogram(
		"log_operation_duration_milliseconds",
		metric.WithDescription("Latency of log operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create latency histogram: %w", err)
	}

	return &PrometheusMetricsProvider{
		meter:         meter,
		logCounter:    logCounter,
		errorCounter:  errorCounter,
		latencyHist:   latencyHist,
		customMetrics: make(map[string]interface{}),
	}, nil
}

func (p *PrometheusMetricsProvider) Log(ctx context.Context, level domain.Level, module string, latency time.Duration) error {
	// Record log count
	attrs := []attribute.KeyValue{
		attribute.String("level", level.String()),
		attribute.String("module", module),
	}

	p.logCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record latency if provided
	if latency > 0 {
		p.latencyHist.Record(ctx, float64(latency.Milliseconds()),
			metric.WithAttributes(attrs...))
	}

	return nil
}

func (p *PrometheusMetricsProvider) Err(ctx context.Context, module string, errorType string) error {
	attrs := []attribute.KeyValue{
		attribute.String("module", module),
		attribute.String("error_type", errorType),
	}

	p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	return nil
}

func (p *PrometheusMetricsProvider) Rec(ctx context.Context, name string, value float64, tags map[string]string) error {
	// Convert tags to attributes
	attrs := make([]attribute.KeyValue, 0, len(tags))
	for k, v := range tags {
		attrs = append(attrs, attribute.String(k, v))
	}

	// Check if metric exists
	p.mu.RLock()
	metricInst, exists := p.customMetrics[name]
	p.mu.RUnlock()

	if !exists {
		// Create new metric
		p.mu.Lock()
		// Double-check after acquiring write lock
		if metricInst, exists = p.customMetrics[name]; !exists {
			gauge, err := p.meter.Float64Gauge(
				name,
				metric.WithDescription(fmt.Sprintf("Custom metric: %s", name)),
			)
			if err != nil {
				p.mu.Unlock()
				return fmt.Errorf("failed to create metric %s: %w", name, err)
			}
			p.customMetrics[name] = gauge
			metricInst = gauge
		}
		p.mu.Unlock()
	}

	// Record value
	if gauge, ok := metricInst.(metric.Float64Gauge); ok {
		gauge.Record(ctx, value, metric.WithAttributes(attrs...))
	}

	return nil
}

func (p *PrometheusMetricsProvider) Get() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metrics := make(map[string]interface{})

	// Add standard metrics info
	metrics["log_counter"] = "log_messages_total"
	metrics["error_counter"] = "log_errors_total"
	metrics["latency_histogram"] = "log_operation_duration_milliseconds"

	// Add custom metrics
	for name := range p.customMetrics {
		metrics[name] = "custom"
	}

	return metrics
}

func (p *PrometheusMetricsProvider) Health() error {
	// Prometheus metrics are always healthy if created
	return nil
}

func (p *PrometheusMetricsProvider) Close() error {
	// Nothing to close for Prometheus metrics
	return nil
}

// SimpleMetricsProvider provides in-memory metrics (for testing or simple use cases)
type SimpleMetricsProvider struct {
	logCounts     map[string]int64
	errorCounts   map[string]int64
	latencies     []time.Duration
	customMetrics map[string]float64
	mu            sync.RWMutex
}

// NewSimpleMetricsProvider creates a new simple metrics provider
func NewSimpleMetricsProvider() outbound.Metrics {
	return &SimpleMetricsProvider{
		logCounts:     make(map[string]int64),
		errorCounts:   make(map[string]int64),
		latencies:     make([]time.Duration, 0),
		customMetrics: make(map[string]float64),
	}
}

func (s *SimpleMetricsProvider) Log(ctx context.Context, level domain.Level, module string, latency time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", level.String(), module)
	s.logCounts[key]++

	if latency > 0 {
		s.latencies = append(s.latencies, latency)
	}

	return nil
}

func (s *SimpleMetricsProvider) Err(ctx context.Context, module string, errorType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", module, errorType)
	s.errorCounts[key]++

	return nil
}

func (s *SimpleMetricsProvider) Rec(ctx context.Context, name string, value float64, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simple implementation: ignore tags for now
	s.customMetrics[name] = value

	return nil
}

func (s *SimpleMetricsProvider) Get() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make(map[string]interface{})

	// Copy log counts
	logCounts := make(map[string]int64)
	for k, v := range s.logCounts {
		logCounts[k] = v
	}
	metrics["log_counts"] = logCounts

	// Copy error counts
	errorCounts := make(map[string]int64)
	for k, v := range s.errorCounts {
		errorCounts[k] = v
	}
	metrics["error_counts"] = errorCounts

	// Calculate latency stats
	if len(s.latencies) > 0 {
		var total time.Duration
		var min, max time.Duration = s.latencies[0], s.latencies[0]

		for _, lat := range s.latencies {
			total += lat
			if lat < min {
				min = lat
			}
			if lat > max {
				max = lat
			}
		}

		metrics["latency_stats"] = map[string]interface{}{
			"count": len(s.latencies),
			"min":   min.Milliseconds(),
			"max":   max.Milliseconds(),
			"avg":   total.Milliseconds() / int64(len(s.latencies)),
		}
	}

	// Copy custom metrics
	customMetrics := make(map[string]float64)
	for k, v := range s.customMetrics {
		customMetrics[k] = v
	}
	metrics["custom_metrics"] = customMetrics

	return metrics
}

func (s *SimpleMetricsProvider) Health() error {
	return nil
}

func (s *SimpleMetricsProvider) Close() error {
	return nil
}

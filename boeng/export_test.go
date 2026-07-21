package boeng

// ExtractFieldsForTest exposes the internal field extractor to the *_test.go
// files in this directory's _test package. Test-only.
var ExtractFieldsForTest = extractFields

// CamelToSnakeForTest exposes the snake_case converter for direct table tests.
var CamelToSnakeForTest = camelToSnake

// SanitizeMetricNameForTest exposes the metric-name sanitizer for table tests.
var SanitizeMetricNameForTest = sanitizeMetricName

// MetricLabelsForTest exposes metricLabels for cardinality-guard tests.
var MetricLabelsForTest = metricLabels

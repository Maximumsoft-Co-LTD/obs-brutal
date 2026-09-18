package otel

import "testing"

// Config.OTel follows the OTel exporter spec: a URL whose scheme decides
// transport security. Bare host:port stays accepted (v1.x compatibility)
// and means plaintext, exactly what boeng did before.
func TestResolveEndpoint(t *testing.T) {
	cases := []struct {
		in       string
		host     string
		insecure bool
	}{
		{"", "", true},
		{"otel-collector:4317", "otel-collector:4317", true},
		{"10.0.0.1:4317", "10.0.0.1:4317", true},
		{"http://otel-collector:4317", "otel-collector:4317", true},
		{"https://otel-collector:4317", "otel-collector:4317", false},
		{"https://otel.example.com", "otel.example.com", false},
		{"http://otel-collector:4317/", "otel-collector:4317", true},
	}
	for _, c := range cases {
		host, insecure := resolveEndpoint(c.in)
		if host != c.host || insecure != c.insecure {
			t.Errorf("resolveEndpoint(%q) = (%q, %v), want (%q, %v)", c.in, host, insecure, c.host, c.insecure)
		}
	}
}

func TestExportConfigured(t *testing.T) {
	for _, k := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"} {
		t.Setenv(k, "")
	}
	if ExportConfigured("") {
		t.Error("no endpoint, no env: must not export")
	}
	if !ExportConfigured("http://c:4317") {
		t.Error("explicit endpoint must export")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://c:4317")
	if !ExportConfigured("") {
		t.Error("OTEL_EXPORTER_OTLP_ENDPOINT alone must enable export")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://c:4317")
	if !ExportConfigured("") {
		t.Error("signal-specific env must enable export")
	}
}

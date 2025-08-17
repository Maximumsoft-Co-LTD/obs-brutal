package obsvbrutal

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ValidateOTelConfig validates OpenTelemetry configuration
func ValidateOTelConfig(enabled bool, endpoint string) error {
	if !enabled {
		return nil // Skip validation if disabled
	}

	if endpoint == "" {
		return fmt.Errorf("OTEL endpoint is required when enabled")
	}

	// Try to connect to endpoint with timeout
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return validateHTTPEndpoint(endpoint)
	} else {
		return validateGRPCEndpoint(endpoint)
	}
}

func validateHTTPEndpoint(endpoint string) error {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("HTTP endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func validateGRPCEndpoint(endpoint string) error {
	conn, err := net.DialTimeout("tcp", endpoint, 3*time.Second)
	if err != nil {
		return fmt.Errorf("gRPC endpoint unreachable: %w", err)
	}
	defer conn.Close()

	return nil
}

// ValidatePrometheusConfig validates Prometheus configuration
func ValidatePrometheusConfig(enabled bool, endpoint string) error {
	if !enabled || endpoint == "" {
		return nil // Skip validation if disabled or empty
	}

	return validateHTTPEndpoint(endpoint)
}

// ValidateLokiConfig validates Loki configuration
func ValidateLokiConfig(enabled bool, endpoint string) error {
	if !enabled || endpoint == "" {
		return nil // Skip validation if disabled or empty
	}

	// Test Loki ready endpoint
	readyURL := strings.TrimSuffix(endpoint, "/") + "/ready"
	return validateHTTPEndpoint(readyURL)
}

// SafeInitOTel initializes OpenTelemetry with error handling
func SafeInitOTel(serviceName, endpoint string, insecure bool) (*OTelProvider, error) {
	if endpoint == "" {
		fmt.Println("Info: OpenTelemetry endpoint not configured. Tracing disabled.")
		return &OTelProvider{
			tracerProvider: nil,
			meterProvider:  nil,
			tracer:         nil,
			meter:          nil,
			propagator:     nil,
		}, nil
	}

	// Validate endpoint before attempting connection
	if err := ValidateOTelConfig(true, endpoint); err != nil {
		fmt.Printf("Warning: OpenTelemetry validation failed: %v. Tracing disabled.\n", err)
		return &OTelProvider{
			tracerProvider: nil,
			meterProvider:  nil,
			tracer:         nil,
			meter:          nil,
			propagator:     nil,
		}, nil
	}

	// Try to initialize with timeout
	provider, err := NewOTelProvider(serviceName, endpoint, insecure)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize OpenTelemetry: %v. Tracing disabled.\n", err)
		return &OTelProvider{
			tracerProvider: nil,
			meterProvider:  nil,
			tracer:         nil,
			meter:          nil,
			propagator:     nil,
		}, nil
	}

	fmt.Println("Info: OpenTelemetry initialized successfully.")
	return provider, nil
}

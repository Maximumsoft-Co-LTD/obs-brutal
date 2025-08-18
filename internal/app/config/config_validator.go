package config

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	obsvx "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/shared"
)

// ValidateOTelConfig validates OpenTelemetry configuration
func ValidateOTelConfig(enabled bool, endpoint string) error {
	if !enabled {
		return nil
	}
	if endpoint == "" {
		return fmt.Errorf("OTEL endpoint is required when enabled")
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return validateHTTPEndpoint(endpoint)
	} else {
		return validateGRPCEndpoint(endpoint)
	}
}

func validateHTTPEndpoint(endpoint string) error {
	client := &http.Client{Timeout: 3 * time.Second}
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
		return nil
	}
	return validateHTTPEndpoint(endpoint)
}

// ValidateLokiConfig validates Loki configuration
func ValidateLokiConfig(enabled bool, endpoint string) error {
	if !enabled || endpoint == "" {
		return nil
	}
	readyURL := strings.TrimSuffix(endpoint, "/") + "/ready"
	return validateHTTPEndpoint(readyURL)
}

// SafeInitOTel initializes OpenTelemetry with error handling
func SafeInitOTel(serviceName, endpoint string, insecure bool) (*obsvx.OTelProvider, error) {
	if endpoint == "" {
		fmt.Println("Info: OpenTelemetry endpoint not configured. Tracing disabled.")
		return nil, nil
	}
	if err := ValidateOTelConfig(true, endpoint); err != nil {
		fmt.Printf("Warning: OpenTelemetry validation failed: %v. Tracing disabled.\n", err)
		return nil, nil
	}
	provider, err := obsvx.NewOTelProvider(serviceName, endpoint, insecure)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize OpenTelemetry: %v. Tracing disabled.\n", err)
		return nil, nil
	}
	fmt.Println("Info: OpenTelemetry initialized successfully.")
	return provider, nil
}

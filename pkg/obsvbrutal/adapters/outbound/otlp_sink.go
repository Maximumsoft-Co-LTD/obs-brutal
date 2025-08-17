package outbound

import (
	"context"
	"fmt"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/outbound"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OTLPSink sends logs to OpenTelemetry collector
type OTLPSink struct {
	endpoint string
	insecure bool
	conn     *grpc.ClientConn
}

// NewOTLPSink creates a new OTLP sink
func NewOTLPSink(endpoint string, insecureConn bool) (outbound.Sink, error) {
	return &OTLPSink{
		endpoint: endpoint,
		insecure: insecureConn,
	}, nil
}

func (s *OTLPSink) Write(entry *domain.LogEntry) error {
	// TODO: Implement OTLP log export when the log SDK is stable
	// For now, this is a placeholder
	return nil
}

func (s *OTLPSink) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *OTLPSink) Name() string {
	return "otlp"
}

func (s *OTLPSink) Health() error {
	// Try to establish connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, s.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to OTLP endpoint: %w", err)
	}

	conn.Close()
	return nil
}

package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/outbound"
)

// LokiSink sends logs to Loki
type LokiSink struct {
	url       string
	labels    map[string]string
	client    *http.Client
	batchSize int
	batch     []lokiEntry
	mu        sync.Mutex
}

type lokiEntry struct {
	timestamp string
	line      string
}

type lokiPushRequest struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// NewLokiSink creates a new Loki sink
func NewLokiSink(url string, labels map[string]string, batchSize int) outbound.Sink {
	return &LokiSink{
		url:       url,
		labels:    labels,
		client:    &http.Client{Timeout: 30 * time.Second},
		batchSize: batchSize,
		batch:     make([]lokiEntry, 0, batchSize),
	}
}

func (s *LokiSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	// Format entry as JSON
	data, err := json.Marshal(map[string]interface{}{
		"level":      entry.Level.String(),
		"message":    entry.Message,
		"fields":     entry.Fields,
		"timestamp":  entry.Timestamp.Format(time.RFC3339Nano),
		"trace_id":   entry.TraceID,
		"span_id":    entry.SpanID,
		"request_id": entry.RequestID,
		"user_id":    entry.UserID,
		"tenant_id":  entry.TenantID,
		"module":     entry.Module,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	lokiEnt := lokiEntry{
		timestamp: strconv.FormatInt(entry.Timestamp.UnixNano(), 10),
		line:      string(data),
	}

	s.mu.Lock()
	s.batch = append(s.batch, lokiEnt)

	// Check if batch is full
	if len(s.batch) >= s.batchSize {
		batch := s.batch
		s.batch = make([]lokiEntry, 0, s.batchSize)
		s.mu.Unlock()

		return s.sendBatch(batch)
	}

	s.mu.Unlock()
	return nil
}

func (s *LokiSink) sendBatch(batch []lokiEntry) error {
	// Build Loki push request
	values := make([][]string, 0, len(batch))
	for _, entry := range batch {
		values = append(values, []string{entry.timestamp, entry.line})
	}

	pushReq := lokiPushRequest{
		Streams: []lokiStream{
			{
				Stream: s.labels,
				Values: values,
			},
		},
	}

	// Marshal request
	data, err := json.Marshal(pushReq)
	if err != nil {
		return fmt.Errorf("failed to marshal push request: %w", err)
	}

	// Send to Loki
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.url+"/loki/api/v1/push", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Loki returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *LokiSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send remaining batch
	if len(s.batch) > 0 {
		return s.sendBatch(s.batch)
	}

	return nil
}

func (s *LokiSink) Name() string {
	return "loki"
}

func (s *LokiSink) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/ready", nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Loki health check failed: %d", resp.StatusCode)
	}

	return nil
}

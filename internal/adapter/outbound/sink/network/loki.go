// Package network contains sinks that push entries over the network.
package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"obs-brutal/internal/util"
	"time"
)

// LokiPushSink pushes logs to Grafana Loki via the /loki/api/v1/push endpoint.
// Expected labels are provided in the constructor or via Configure.
type LokiPushSink struct {
	port.SinkBase
	endpoint string
	labels   map[string]string
	client   *http.Client
}

// NewLokiPushSink creates a Loki push sink. If labels is nil, a default
// label set {app: "obs-brutal"} is used.
func NewLokiPushSink(endpoint string, labels map[string]string) port.Sink {
	if labels == nil {
		labels = map[string]string{"app": "obs-brutal"}
	}
	return &LokiPushSink{endpoint: endpoint, labels: labels, client: &http.Client{Timeout: 5 * time.Second}}
}
func (s *LokiPushSink) Name() string { return "loki" }
// Configure supports keys: "endpoint" (string), "labels" (map[string]string).
func (s *LokiPushSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := cfg["labels"].(map[string]string); ok {
		s.labels = v
	}
	return nil
}
func (s *LokiPushSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.endpoint == "" {
		return nil
	}
	lineBytes, err := util.EncodeEntryToJSON(entry)
	if err != nil {
		return err
	}
	ts := entry.Timestamp.UnixNano()
	payload := map[string]interface{}{"streams": []map[string]interface{}{{"stream": s.labels, "values": [][]string{{fmt.Sprintf("%d", ts), string(lineBytes)}}}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("loki push failed: %s", resp.Status)
	}
	return nil
}

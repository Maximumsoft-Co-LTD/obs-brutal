// Package alerts contains sinks that send alert notifications (e.g., Opsgenie).
package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"time"
)

type OpsgenieSink struct {
	port.SinkBase
	apiKey, endpoint, priority string
	client                     *http.Client
}

func NewOpsgenieSink(apiKey string) port.Sink {
	return &OpsgenieSink{apiKey: apiKey, endpoint: "https://api.opsgenie.com/v2/alerts", client: &http.Client{Timeout: 5 * time.Second}, priority: "P3"}
}
func (s *OpsgenieSink) Name() string { return "opsgenie" }
func (s *OpsgenieSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["api_key"].(string); ok {
		s.apiKey = v
	}
	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := cfg["priority"].(string); ok && v != "" {
		s.priority = v
	}
	return nil
}
func (s *OpsgenieSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.apiKey == "" {
		return nil
	}
	payload := map[string]interface{}{"message": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Msg), "priority": s.priority}
	body, _ := json.Marshal(payload)
	return retryBackoff(func() error {
		req, _ := http.NewRequest("POST", s.endpoint, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "GenieKey "+s.apiKey)
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("opsgenie status: %s", resp.Status)
		}
		return nil
	})
}

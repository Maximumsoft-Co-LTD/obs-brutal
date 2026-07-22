// Package alerts contains sinks that send alert notifications (e.g., Opsgenie).
package alerts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
)

type OpsgenieSink struct {
	port.SinkBase
	mu                         sync.RWMutex
	apiKey, endpoint, priority string
	client                     *http.Client
}

func NewOpsgenieSink(apiKey string) port.Sink {
	return &OpsgenieSink{apiKey: apiKey, endpoint: "https://api.opsgenie.com/v2/alerts", client: &http.Client{Timeout: 5 * time.Second}, priority: "P3"}
}
func (s *OpsgenieSink) Name() string { return "opsgenie" }
func (s *OpsgenieSink) Configure(cfg map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if entry == nil {
		return nil
	}
	s.mu.RLock()
	apiKey, endpoint, priority := s.apiKey, s.endpoint, s.priority
	s.mu.RUnlock()
	if apiKey == "" {
		return nil
	}
	payload := map[string]interface{}{"message": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Msg), "priority": priority}
	body, _ := json.Marshal(payload)
	return deliver(s.client, postConfig{
		name:    "opsgenie",
		url:     endpoint,
		headers: map[string]string{"Authorization": "GenieKey " + apiKey},
		body:    body,
	})
}

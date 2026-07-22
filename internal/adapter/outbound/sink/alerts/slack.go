// Package alerts contains sinks that send alert notifications (e.g., Slack).
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

type SlackSink struct {
	port.SinkBase
	mu         sync.RWMutex
	webhookURL string
	client     *http.Client
}

func NewSlackSink(webhookURL string) port.Sink {
	return &SlackSink{webhookURL: webhookURL, client: &http.Client{Timeout: 5 * time.Second}}
}

// Name returns the sink name.
func (s *SlackSink) Name() string { return "slack" }

func (s *SlackSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["webhook_url"].(string); ok && v != "" {
		s.mu.Lock()
		s.webhookURL = v
		s.mu.Unlock()
	}
	return nil
}

func (s *SlackSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.mu.RLock()
	url := s.webhookURL
	s.mu.RUnlock()
	if url == "" {
		return nil
	}
	payload := map[string]interface{}{"text": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Msg)}
	body, _ := json.Marshal(payload)
	return deliver(s.client, postConfig{name: "slack", url: url, body: body})
}

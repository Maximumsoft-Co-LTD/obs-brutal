// Package alerts contains sinks that send alert notifications (e.g., Slack).
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

// simple retry with exponential backoff
func retryBackoff(op func() error) error {
	delay := 200 * time.Millisecond
	for attempt := 0; attempt < 5; attempt++ {
		if err := op(); err != nil {
			if attempt == 4 {
				return err
			}
			time.Sleep(delay)
			if delay < 2*time.Second {
				delay *= 2
			}
			continue
		}
		return nil
	}
	return nil
}

type SlackSink struct {
	port.SinkBase
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
		s.webhookURL = v
	}
	return nil
}
func (s *SlackSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.webhookURL == "" {
		return nil
	}
	payload := map[string]interface{}{"text": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Msg)}
	body, _ := json.Marshal(payload)
	return retryBackoff(func() error {
		req, _ := http.NewRequest("POST", s.webhookURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("slack status: %s", resp.Status)
		}
		return nil
	})
}

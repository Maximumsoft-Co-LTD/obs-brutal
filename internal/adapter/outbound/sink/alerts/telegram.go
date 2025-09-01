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

type TelegramSink struct {
	port.SinkBase
	botToken, chatID string
	client           *http.Client
}

func NewTelegramSink(botToken, chatID string) port.Sink {
	return &TelegramSink{botToken: botToken, chatID: chatID, client: &http.Client{Timeout: 5 * time.Second}}
}
func (s *TelegramSink) Name() string { return "telegram" }
func (s *TelegramSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["bot_token"].(string); ok {
		s.botToken = v
	}
	if v, ok := cfg["chat_id"].(string); ok {
		s.chatID = v
	}
	return nil
}
func (s *TelegramSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.botToken == "" || s.chatID == "" {
		return nil
	}
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	data := map[string]string{"chat_id": s.chatID, "text": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Msg)}
	body, _ := json.Marshal(data)
	return retryBackoff(func() error {
		req, _ := http.NewRequest("POST", api, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("telegram status: %s", resp.Status)
		}
		return nil
	})
}

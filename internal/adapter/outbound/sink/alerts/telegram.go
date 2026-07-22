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

type TelegramSink struct {
	port.SinkBase
	mu               sync.RWMutex
	botToken, chatID string
	client           *http.Client
}

func NewTelegramSink(botToken, chatID string) port.Sink {
	return &TelegramSink{botToken: botToken, chatID: chatID, client: &http.Client{Timeout: 5 * time.Second}}
}
func (s *TelegramSink) Name() string { return "telegram" }
func (s *TelegramSink) Configure(cfg map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := cfg["bot_token"].(string); ok {
		s.botToken = v
	}
	if v, ok := cfg["chat_id"].(string); ok {
		s.chatID = v
	}
	return nil
}
func (s *TelegramSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.mu.RLock()
	token, chatID := s.botToken, s.chatID
	s.mu.RUnlock()
	if token == "" || chatID == "" {
		return nil
	}
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	data := map[string]string{"chat_id": chatID, "text": fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Msg)}
	body, _ := json.Marshal(data)
	return deliver(s.client, postConfig{name: "telegram", url: api, body: body})
}

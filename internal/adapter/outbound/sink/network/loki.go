// Package network contains sinks that push entries over the network.
package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util"
)

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		if in == nil {
			return nil
		}
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func toStringMap(v interface{}) (map[string]string, bool) {
	switch typed := v.(type) {
	case map[string]string:
		return cloneStringMap(typed), true
	case map[string]interface{}:
		out := make(map[string]string, len(typed))
		for k, val := range typed {
			out[k] = fmt.Sprintf("%v", val)
		}
		return out, true
	default:
		return nil, false
	}
}

func toInt(v interface{}) (int, bool) {
	switch typed := v.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

// LokiPushSink pushes logs to Grafana Loki via the /loki/api/v1/push endpoint.
// Expected labels are provided in the constructor or via Configure.
type LokiPushSink struct {
	port.SinkBase
	endpoint string
	labels   map[string]string
	client   *http.Client
	headers  map[string]string

	// batching
	mu     sync.Mutex
	values [][]string // [[ts, lineJSON]]
	max    int
	tout   time.Duration
	stopCh chan struct{}
	wg     sync.WaitGroup

	// retry
	retries int
	backoff time.Duration
}

// NewLokiPushSink creates a Loki push sink. If labels is nil, a default
// label set {app: "obs-brutal"} is used.
func NewLokiPushSink(endpoint string, labels map[string]string) port.Sink {
	if labels == nil {
		labels = map[string]string{"app": "obs-brutal"}
	}
	labelCopy := cloneStringMap(labels)
	s := &LokiPushSink{
		endpoint: endpoint,
		labels:   labelCopy,
		client:   &http.Client{Timeout: 5 * time.Second},
		headers:  map[string]string{},
		values:   make([][]string, 0, 100),
		max:      100,
		tout:     100 * time.Millisecond,
		retries:  3,
		backoff:  100 * time.Millisecond,
	}
	s.start()
	return s
}
func (s *LokiPushSink) Name() string { return "loki" }

func (s *LokiPushSink) Write(entry *domain.LogEntry) error {
	if entry == nil || s.endpoint == "" {
		return nil
	}

	lineBytes, err := util.EncodeEntryToJSON(entry)
	if err != nil {
		return err
	}
	ts := entry.Timestamp.UnixNano()

	s.mu.Lock()
	s.values = append(s.values, []string{fmt.Sprintf("%d", ts), string(lineBytes)})
	needFlush := s.max > 0 && len(s.values) >= s.max
	s.mu.Unlock()

	if needFlush {
		return s.flush()
	}
	return nil
}

func (s *LokiPushSink) flush() error {
	s.mu.Lock()
	if s.endpoint == "" {
		s.mu.Unlock()
		return nil
	}
	if len(s.values) == 0 {
		s.mu.Unlock()
		return nil
	}

	vals := make([][]string, len(s.values))
	copy(vals, s.values)
	s.values = s.values[:0]

	labelsCopy := cloneStringMap(s.labels)
	headersCopy := cloneStringMap(s.headers)
	endpoint := s.endpoint
	s.mu.Unlock()

	payload := map[string]interface{}{
		"streams": []map[string]interface{}{
			{"stream": labelsCopy, "values": vals},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.mu.Lock()
		s.values = append(vals, s.values...)
		s.mu.Unlock()
		return err
	}

	if err := s.post(endpoint, headersCopy, body); err != nil {
		s.mu.Lock()
		s.values = append(vals, s.values...)
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *LokiPushSink) start() {
	s.mu.Lock()
	if s.stopCh != nil {
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	t := s.tout
	if t <= 0 {
		t = 100 * time.Millisecond
	}
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(t)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				_ = s.flush()
			}
		}
	}()
}

func (s *LokiPushSink) Close() error {
	s.mu.Lock()
	if s.stopCh != nil {
		close(s.stopCh)
	}
	s.mu.Unlock()

	s.wg.Wait()
	return s.flush()
}

func (s *LokiPushSink) Configure(cfg map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := toStringMap(cfg["labels"]); ok {
		s.labels = v
	}
	if v, ok := toInt(cfg["batch_size"]); ok && v > 0 {
		s.max = v
	}
	if v, ok := toInt(cfg["timeout_ms"]); ok && v > 0 {
		s.tout = time.Duration(v) * time.Millisecond
	}
	if v, ok := toInt(cfg["max_retries"]); ok && v >= 0 {
		s.retries = v
	}
	if v, ok := toInt(cfg["retry_base_ms"]); ok && v > 0 {
		s.backoff = time.Duration(v) * time.Millisecond
	}
	if v, ok := toStringMap(cfg["headers"]); ok {
		s.headers = v
	}
	return nil
}

func (s *LokiPushSink) post(endpoint string, headers map[string]string, body []byte) error {
	var lastErr error
	tries := s.retries
	if tries < 0 {
		tries = 0
	}

	for i := 0; i <= tries; i++ {
		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		// req.Header.Set("Content-Encoding", "gzip") // if gzipped
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := s.client.Do(req)
		if err == nil {
			code := resp.StatusCode
			_ = resp.Body.Close()
			if code < 300 {
				return nil
			}
			if code != 429 && code < 500 {
				return fmt.Errorf("loki push failed: %d", code)
			}
			lastErr = fmt.Errorf("loki push failed: %d", code)
		} else {
			lastErr = err
		}

		d := s.backoff
		if d <= 0 {
			d = 100 * time.Millisecond
		}
		// jitter 0–100ms
		jitter := time.Duration(time.Now().UnixNano() % int64(100*time.Millisecond))
		time.Sleep(d*time.Duration(1<<uint(i)) + jitter)
	}
	return lastErr
}

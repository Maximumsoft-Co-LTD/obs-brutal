// Package network contains sinks that push entries over the network.
package network

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "obs-brutal/internal/core/domain"
    "obs-brutal/internal/core/port"
    "obs-brutal/internal/util"
)

// LokiPushSink pushes logs to Grafana Loki via the /loki/api/v1/push endpoint.
// Expected labels are provided in the constructor or via Configure.
type LokiPushSink struct {
    port.SinkBase
    endpoint string
    labels   map[string]string
    client   *http.Client
    headers  map[string]string

    // batching
    mu       sync.Mutex
    values   [][]string // [[ts, lineJSON]]
    max      int
    tout     time.Duration
    stopCh   chan struct{}
    wg       sync.WaitGroup

    // retry
    retries  int
    backoff  time.Duration
}

// NewLokiPushSink creates a Loki push sink. If labels is nil, a default
// label set {app: "obs-brutal"} is used.
func NewLokiPushSink(endpoint string, labels map[string]string) port.Sink {
    if labels == nil {
        labels = map[string]string{"app": "obs-brutal"}
    }
    s := &LokiPushSink{
        endpoint: endpoint,
        labels:   labels,
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
// Configure supports keys: "endpoint" (string), "labels" (map[string]string).
func (s *LokiPushSink) Configure(cfg map[string]interface{}) error {
    if v, ok := cfg["endpoint"].(string); ok && v != "" {
        s.endpoint = v
    }
    if v, ok := cfg["labels"].(map[string]string); ok {
        s.labels = v
    }
    if v, ok := cfg["batch_size"].(int); ok && v > 0 { s.max = v }
    if v, ok := cfg["timeout_ms"].(int); ok && v > 0 { s.tout = time.Duration(v) * time.Millisecond }
    if v, ok := cfg["max_retries"].(int); ok && v >= 0 { s.retries = v }
    if v, ok := cfg["retry_base_ms"].(int); ok && v > 0 { s.backoff = time.Duration(v) * time.Millisecond }
    if v, ok := cfg["headers"].(map[string]string); ok { s.headers = v }
    return nil
}
func (s *LokiPushSink) Write(entry *domain.LogEntry) error {
    if entry == nil || s.endpoint == "" {
        return nil
    }
    lineBytes, err := util.EncodeEntryToJSON(entry)
    if err != nil { return err }
    ts := entry.Timestamp.UnixNano()

    s.mu.Lock()
    s.values = append(s.values, []string{fmt.Sprintf("%d", ts), string(lineBytes)})
    needFlush := s.max > 0 && len(s.values) >= s.max
    if needFlush {
        _ = s.flushLocked()
    }
    s.mu.Unlock()
    return nil
}

func (s *LokiPushSink) start() {
    s.mu.Lock()
    if s.stopCh != nil { s.mu.Unlock(); return }
    s.stopCh = make(chan struct{})
    t := s.tout; if t <= 0 { t = 100 * time.Millisecond }
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
                s.mu.Lock()
                if len(s.values) > 0 { _ = s.flushLocked() }
                s.mu.Unlock()
            }
        }
    }()
}

func (s *LokiPushSink) Close() error {
    s.mu.Lock()
    if s.stopCh != nil { close(s.stopCh) }
    s.mu.Unlock()
    s.wg.Wait()
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.flushLocked()
}

func (s *LokiPushSink) flushLocked() error {
    if s.endpoint == "" || len(s.values) == 0 { return nil }
    // build payload with current buffer
    vals := make([][]string, len(s.values))
    copy(vals, s.values)
    s.values = s.values[:0]
    payload := map[string]interface{}{
        "streams": []map[string]interface{}{
            {"stream": s.labels, "values": vals},
        },
    }
    body, err := json.Marshal(payload)
    if err != nil { return err }
    return s.post(body)
}

func (s *LokiPushSink) post(body []byte) error {
    var lastErr error
    tries := s.retries
    if tries < 0 { tries = 0 }
    for i := 0; i <= tries; i++ {
        req, err := http.NewRequest("POST", s.endpoint, bytes.NewReader(body))
        if err != nil { return err }
        req.Header.Set("Content-Type", "application/json")
        for k, v := range s.headers { req.Header.Set(k, v) }
        resp, err := s.client.Do(req)
        if err != nil { lastErr = err } else {
            code := resp.StatusCode
            _ = resp.Body.Close()
            if code < 300 { return nil }
            // retry on 429/5xx
            if code != 429 && code < 500 { return fmt.Errorf("loki push failed: %d", code) }
            lastErr = fmt.Errorf("loki push failed: %d", code)
        }
        // backoff
        d := s.backoff
        if d <= 0 { d = 100 * time.Millisecond }
        time.Sleep(d * time.Duration(1<<uint(i)))
    }
    return lastErr
}

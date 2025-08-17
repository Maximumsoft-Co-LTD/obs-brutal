package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/outbound"
	"os"
	"sync"
	"time"

	"github.com/natefinch/lumberjack"
)

// StdoutSink writes logs to stdout
type StdoutSink struct {
	formatter outbound.Formatter
	mu        sync.Mutex
}

// NewStdoutSink creates a new stdout sink
func NewStdoutSink(formatter outbound.Formatter) outbound.Sink {
	if formatter == nil {
		formatter = NewJSONFormatter()
	}
	return &StdoutSink{
		formatter: formatter,
	}
}

func (s *StdoutSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Format entry
	output := s.formatter.Format(entry)

	// Write to stdout
	_, err := fmt.Fprintln(os.Stdout, output)
	return err
}

func (s *StdoutSink) Close() error {
	return nil
}

func (s *StdoutSink) Name() string {
	return "stdout"
}

func (s *StdoutSink) Health() error {
	return nil
}

// FileSink writes logs to a file with rotation
type FileSink struct {
	writer    *lumberjack.Logger
	formatter outbound.Formatter
	mu        sync.Mutex
}

// NewFileSink creates a new file sink
func NewFileSink(filename string, maxSize, maxAge, maxBackups int, compress bool, formatter outbound.Formatter) outbound.Sink {
	if formatter == nil {
		formatter = NewJSONFormatter()
	}

	return &FileSink{
		writer: &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize, // megabytes
			MaxAge:     maxAge,  // days
			MaxBackups: maxBackups,
			Compress:   compress,
		},
		formatter: formatter,
	}
}

func (s *FileSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Format entry
	output := s.formatter.Format(entry)

	// Write to file
	_, err := s.writer.Write([]byte(output + "\n"))
	return err
}

func (s *FileSink) Close() error {
	return s.writer.Close()
}

func (s *FileSink) Name() string {
	return "file"
}

func (s *FileSink) Health() error {
	// Try to write a test entry
	testEntry := &domain.LogEntry{
		Level:     domain.DebugLevel,
		Message:   "health check",
		Timestamp: time.Now(),
	}
	return s.Write(testEntry)
}

// HTTPSink sends logs to HTTP endpoint
type HTTPSink struct {
	url       string
	headers   map[string]string
	client    outbound.HTTPClient
	batchSize int
	batch     []*domain.LogEntry
	mu        sync.Mutex
}

// NewHTTPSink creates a new HTTP sink
func NewHTTPSink(url string, headers map[string]string, client outbound.HTTPClient, batchSize int) outbound.Sink {
	if client == nil {
		client = NewDefaultHTTPClient()
	}

	return &HTTPSink{
		url:       url,
		headers:   headers,
		client:    client,
		batchSize: batchSize,
		batch:     make([]*domain.LogEntry, 0, batchSize),
	}
}

func (s *HTTPSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.Lock()
	s.batch = append(s.batch, entry)

	// Check if batch is full
	if len(s.batch) >= s.batchSize {
		batch := s.batch
		s.batch = make([]*domain.LogEntry, 0, s.batchSize)
		s.mu.Unlock()

		return s.sendBatch(batch)
	}

	s.mu.Unlock()
	return nil
}

func (s *HTTPSink) sendBatch(batch []*domain.LogEntry) error {
	// Convert batch to JSON
	data, err := json.Marshal(map[string]interface{}{
		"logs": batch,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	// Send HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.client.Post(ctx, s.url, s.headers, data)
	if err != nil {
		return fmt.Errorf("failed to send logs: %w", err)
	}

	if resp.Code >= 400 {
		return fmt.Errorf("HTTP error: %d", resp.Code)
	}

	return nil
}

func (s *HTTPSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send remaining batch
	if len(s.batch) > 0 {
		return s.sendBatch(s.batch)
	}

	return nil
}

func (s *HTTPSink) Name() string {
	return "http"
}

func (s *HTTPSink) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to reach the endpoint
	resp, err := s.client.Get(ctx, s.url+"/health", s.headers)
	if err != nil {
		return err
	}

	if resp.Code >= 400 {
		return fmt.Errorf("HTTP health check failed: %d", resp.Code)
	}

	return nil
}

// BufferedSink provides buffering for any sink
type BufferedSink struct {
	sink       outbound.Sink
	buffer     chan *domain.LogEntry
	bufferSize int
	flushTime  time.Duration
	wg         sync.WaitGroup
	done       chan struct{}
}

// NewBufferedSink creates a new buffered sink
func NewBufferedSink(sink outbound.Sink, bufferSize int, flushTime time.Duration) outbound.Sink {
	bs := &BufferedSink{
		sink:       sink,
		buffer:     make(chan *domain.LogEntry, bufferSize),
		bufferSize: bufferSize,
		flushTime:  flushTime,
		done:       make(chan struct{}),
	}

	// Start flusher
	bs.wg.Add(1)
	go bs.flusher()

	return bs
}

func (s *BufferedSink) Write(entry *domain.LogEntry) error {
	select {
	case s.buffer <- entry:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("buffer full")
	}
}

func (s *BufferedSink) flusher() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.flushTime)
	defer ticker.Stop()

	batch := make([]*domain.LogEntry, 0, s.bufferSize)

	for {
		select {
		case entry := <-s.buffer:
			batch = append(batch, entry)

			// Flush if batch is full
			if len(batch) >= s.bufferSize {
				s.flushBatch(batch)
				batch = make([]*domain.LogEntry, 0, s.bufferSize)
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = make([]*domain.LogEntry, 0, s.bufferSize)
			}

		case <-s.done:
			// Final flush
			if len(batch) > 0 {
				s.flushBatch(batch)
			}
			return
		}
	}
}

func (s *BufferedSink) flushBatch(batch []*domain.LogEntry) {
	for _, entry := range batch {
		if err := s.sink.Write(entry); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
		}
	}
}

func (s *BufferedSink) Close() error {
	close(s.done)
	s.wg.Wait()
	return s.sink.Close()
}

func (s *BufferedSink) Name() string {
	return fmt.Sprintf("buffered(%s)", s.sink.Name())
}

func (s *BufferedSink) Health() error {
	return s.sink.Health()
}

// MultiplexSink sends logs to multiple sinks
type MultiplexSink struct {
	sinks []outbound.Sink
	mu    sync.RWMutex
}

// NewMultiplexSink creates a new multiplex sink
func NewMultiplexSink(sinks ...outbound.Sink) outbound.Sink {
	return &MultiplexSink{
		sinks: sinks,
	}
}

func (s *MultiplexSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.RLock()
	sinks := s.sinks
	s.mu.RUnlock()

	var errs []error
	for _, sink := range sinks {
		if err := sink.Write(entry); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sink.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiplex errors: %v", errs)
	}

	return nil
}

func (s *MultiplexSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, sink := range s.sinks {
		if err := sink.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sink.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiplex close errors: %v", errs)
	}

	return nil
}

func (s *MultiplexSink) Name() string {
	return "multiplex"
}

func (s *MultiplexSink) Health() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var errs []error
	for _, sink := range s.sinks {
		if err := sink.Health(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sink.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiplex health errors: %v", errs)
	}

	return nil
}

// DefaultHTTPClient implements HTTPClient interface
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient creates a default HTTP client
func NewDefaultHTTPClient() outbound.HTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *DefaultHTTPClient) Post(ctx context.Context, url string, headers map[string]string, body []byte) (*outbound.HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &outbound.HTTPResponse{
		Code: resp.StatusCode,
		Hdrs: convertHeaders(resp.Header),
		Body: respBody,
	}, nil
}

func (c *DefaultHTTPClient) Get(ctx context.Context, url string, headers map[string]string) (*outbound.HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &outbound.HTTPResponse{
		Code: resp.StatusCode,
		Hdrs: convertHeaders(resp.Header),
		Body: respBody,
	}, nil
}

func convertHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

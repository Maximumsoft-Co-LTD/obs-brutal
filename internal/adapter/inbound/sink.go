package inbound

import (
	"encoding/json"
	"fmt"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/outbound"

	"os"
	"sync"
)

// StdoutSinkAdapter writes logs to stdout
type StdoutSinkAdapter struct {
	mu        sync.Mutex
	formatter outbound.Formatter
}

// NewStdoutSinkAdapter creates a new stdout sink
func NewStdoutSinkAdapter() outbound.Sink {
	return &StdoutSinkAdapter{
		formatter: NewJSONFormatterAdapter(),
	}
}

func (s *StdoutSinkAdapter) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Format entry
	var output string
	if s.formatter != nil {
		output = s.formatter.Format(entry)
	} else {
		// Default JSON formatting
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %w", err)
		}
		output = string(data)
	}

	// Write to stdout
	fmt.Fprintln(os.Stdout, output)

	return nil
}

func (s *StdoutSinkAdapter) Close() error {
	// Nothing to close for stdout
	return nil
}

func (s *StdoutSinkAdapter) Name() string {
	return "stdout"
}

func (s *StdoutSinkAdapter) Health() error {
	return nil
}

// JSONFormatterAdapter formats logs as JSON
type JSONFormatterAdapter struct{}

// NewJSONFormatterAdapter creates a new JSON formatter
func NewJSONFormatterAdapter() outbound.Formatter {
	return &JSONFormatterAdapter{}
}

func (f *JSONFormatterAdapter) Configure(config map[string]interface{}) error {
	return nil
}

func (f *JSONFormatterAdapter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return "{}"
	}

	// Build JSON object
	obj := make(map[string]interface{})

	// Add standard fields
	obj["timestamp"] = entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00")
	obj["level"] = entry.Level.String()
	obj["message"] = entry.Message

	// Add custom fields
	for k, v := range entry.Fields {
		obj[k] = v
	}

	// Add correlation IDs if present
	if entry.TraceID != "" {
		obj["trace_id"] = entry.TraceID
	}
	if entry.SpanID != "" {
		obj["span_id"] = entry.SpanID
	}
	if entry.RequestID != "" {
		obj["request_id"] = entry.RequestID
	}
	if entry.UserID != "" {
		obj["user_id"] = entry.UserID
	}
	if entry.TenantID != "" {
		obj["tenant_id"] = entry.TenantID
	}
	if entry.Module != "" {
		obj["module"] = entry.Module
	}

	// Add error if present
	if entry.Error != nil {
		obj["error"] = entry.Error.Error()
	}

	// Marshal to JSON
	data, err := json.Marshal(obj)
	if err != nil {
		// Fallback to simple format
		return fmt.Sprintf(`{"level":"%s","message":"%s","error":"failed to marshal: %v"}`,
			entry.Level.String(), entry.Message, err)
	}

	return string(data)
}

func (f *JSONFormatterAdapter) Name() string {
	return "json"
}

// TextFormatterAdapter formats logs as text
type TextFormatterAdapter struct{}

// NewTextFormatterAdapter creates a new text formatter
func NewTextFormatterAdapter() outbound.Formatter {
	return &TextFormatterAdapter{}
}

func (f *TextFormatterAdapter) Configure(config map[string]interface{}) error {
	return nil
}

func (f *TextFormatterAdapter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}

	// Build text format
	text := fmt.Sprintf("[%s] %s %s",
		entry.Timestamp.Format("2006-01-02 15:04:05"),
		entry.Level.String(),
		entry.Message,
	)

	// Add fields
	if len(entry.Fields) > 0 {
		text += " {"
		first := true
		for k, v := range entry.Fields {
			if !first {
				text += ", "
			}
			text += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		text += "}"
	}

	// Add error
	if entry.Error != nil {
		text += fmt.Sprintf(" error=%v", entry.Error)
	}

	return text
}

func (f *TextFormatterAdapter) Name() string {
	return "text"
}

// MultiplexSinkAdapter sends logs to multiple sinks
type MultiplexSinkAdapter struct {
	sinks []outbound.Sink
	mu    sync.RWMutex
}

// NewMultiplexSinkAdapter creates a new multiplex sink
func NewMultiplexSinkAdapter(sinks ...outbound.Sink) outbound.Sink {
	return &MultiplexSinkAdapter{
		sinks: sinks,
	}
}

func (m *MultiplexSinkAdapter) Configure(config map[string]interface{}) error {
	return nil
}

func (m *MultiplexSinkAdapter) Health() error {
	return nil
}

func (m *MultiplexSinkAdapter) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}

	m.mu.RLock()
	sinks := m.sinks
	m.mu.RUnlock()

	var errs []error
	for _, sink := range sinks {
		if sink != nil {
			if err := sink.Write(entry); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", sink.Name(), err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiplex sink errors: %v", errs)
	}

	return nil
}

func (m *MultiplexSinkAdapter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for _, sink := range m.sinks {
		if sink != nil {
			if err := sink.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", sink.Name(), err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiplex sink close errors: %v", errs)
	}

	return nil
}

func (m *MultiplexSinkAdapter) Name() string {
	return "multiplex"
}

// AddSink adds a sink to the multiplex
func (m *MultiplexSinkAdapter) AddSink(sink outbound.Sink) {
	if sink == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sinks = append(m.sinks, sink)
}

// RemoveSink removes a sink from the multiplex
func (m *MultiplexSinkAdapter) RemoveSink(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newSinks := make([]outbound.Sink, 0, len(m.sinks))
	for _, sink := range m.sinks {
		if sink != nil && sink.Name() != name {
			newSinks = append(newSinks, sink)
		}
	}
	m.sinks = newSinks
}

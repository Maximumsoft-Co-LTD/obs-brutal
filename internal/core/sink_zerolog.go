package core

import (
	"os"
	"sync"

	"obs-brutal/internal/core/domain"

	"github.com/rs/zerolog"
)

// ZerologSink uses rs/zerolog for zero-alloc style JSON logging.
// It can be used as a drop-in Sink to reduce allocations in hot paths.
type ZerologSink struct {
	SinkBase
	logger zerolog.Logger
	mu     sync.Mutex
}

func NewZerologSink() *ZerologSink {
	zl := zerolog.New(os.Stdout)
	return &ZerologSink{logger: zl}
}

func (s *ZerologSink) Name() string { return "zerolog" }

func (s *ZerologSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	// Build event without extra allocations where possible
	s.mu.Lock()
	evt := s.logger.With().Timestamp().Logger()
	e := evt.Log()
	// Level mapping
	switch entry.Level {
	case domain.DebugLevel:
		e = evt.Debug()
	case domain.InfoLevel:
		e = evt.Info()
	case domain.WarnLevel:
		e = evt.Warn()
	case domain.ErrorLevel:
		e = evt.Error()
	default:
		e = evt.Log()
	}

	e = e.Str("msg", entry.Message)
	if entry.TraceID != "" {
		e = e.Str("trace_id", entry.TraceID)
	}
	if entry.SpanID != "" {
		e = e.Str("span_id", entry.SpanID)
	}
	if entry.RequestID != "" {
		e = e.Str("request_id", entry.RequestID)
	}
	for k, v := range entry.Fields {
		switch tv := v.(type) {
		case string:
			e = e.Str(k, tv)
		case bool:
			e = e.Bool(k, tv)
		case int:
			e = e.Int(k, tv)
		case int64:
			e = e.Int64(k, tv)
		case float64:
			e = e.Float64(k, tv)
		default:
			e = e.Interface(k, v)
		}
	}
	e.Msg("")
	s.mu.Unlock()
	return nil
}

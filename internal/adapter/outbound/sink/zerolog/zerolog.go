package zerolog

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"os"
	"sync"

	"github.com/rs/zerolog"
)

type ZerologSink struct {
	port.SinkBase
	log zerolog.Logger
	mu  sync.Mutex
}

func NewZerologSink() port.Sink     { zl := zerolog.New(os.Stdout); return &ZerologSink{log: zl} }
func (s *ZerologSink) Name() string { return "zerolog" }
func (s *ZerologSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.mu.Lock()
	evt := s.log.With().Timestamp().Logger()
	var e *zerolog.Event
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
	e = e.Str("msg", entry.Msg)
	if entry.TraceID != "" {
		e = e.Str("trace_id", entry.TraceID)
	}
	if entry.SpanID != "" {
		e = e.Str("span_id", entry.SpanID)
	}
	if entry.RequestID != "" {
		e = e.Str("request_id", entry.RequestID)
	}
	for k, v := range entry.F {
		e = e.Interface(k, v)
	}
	e.Msg("")
	s.mu.Unlock()
	return nil
}

package stdout

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"obs-brutal/internal/util"
	"os"
)

type JSONSink struct{ port.SinkBase }

func NewJSONSink() port.Sink     { return &JSONSink{} }
func (s *JSONSink) Name() string { return "json" }
func (s *JSONSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	return util.WriteJSONToWriter(os.Stdout, entry)
}

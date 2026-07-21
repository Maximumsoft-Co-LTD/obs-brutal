package stdout

import (
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util"
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

// Package stdout provides a fast JSON stdout sink suitable for development
// and local benchmarks. It writes each LogEntry as a single JSON line.
package stdout

import (
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util"
	"os"
)

// FastStdoutSink writes LogEntry to os.Stdout in JSON lines.
// Configure is a no-op; Name returns "stdout".
type FastStdoutSink struct{ port.SinkBase }

func NewFastStdoutSink() port.Sink { return &FastStdoutSink{} }

// Name returns the sink name.
func (s *FastStdoutSink) Name() string { return "stdout" }
func (s *FastStdoutSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	return util.WriteJSONToWriter(os.Stdout, entry)
}

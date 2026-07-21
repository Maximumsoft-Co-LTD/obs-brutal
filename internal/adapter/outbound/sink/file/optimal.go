// Package file provides file-based sinks with sensible defaults
// for production (e.g., rotation via lumberjack) and development.
package file

import (
	"fmt"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util"
	"os"
	"path/filepath"
	"sync"
)

type OptimalFileSink struct {
	port.SinkBase
	filename          string
	file              *os.File
	mu                sync.Mutex
	rotateBySizeBytes int64
	maxBackups        int
}

func NewOptimalFileSink() port.Sink     { return &OptimalFileSink{filename: "logs/app.log"} }
func (s *OptimalFileSink) Name() string { return "file" }
func (s *OptimalFileSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["filename"].(string); ok && v != "" {
		s.filename = v
	}
	if v, ok := cfg["rotate_size_bytes"].(int64); ok && v > 0 {
		s.rotateBySizeBytes = v
	}
	if v, ok := cfg["max_backups"].(int); ok && v > 0 {
		s.maxBackups = v
	}
	return nil
}
func (s *OptimalFileSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		dir := filepath.Dir(s.filename)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.OpenFile(s.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		s.file = f
	}
	if s.rotateBySizeBytes > 0 {
		if fi, err := s.file.Stat(); err == nil && fi.Size() >= s.rotateBySizeBytes {
			_ = s.file.Close()
			s.rotateFiles()
			f, err := os.OpenFile(s.filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			s.file = f
		}
	}
	return util.WriteJSONToWriter(s.file, entry)
}
func (s *OptimalFileSink) rotateFiles() {
	for i := s.maxBackups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", s.filename, i), fmt.Sprintf("%s.%d", s.filename, i+1))
	}
	_ = os.Rename(s.filename, fmt.Sprintf("%s.%d", s.filename, 1))
}

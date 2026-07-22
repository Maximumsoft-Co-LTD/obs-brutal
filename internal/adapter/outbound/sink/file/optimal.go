// Package file provides file-based sinks with sensible defaults
// for production (e.g., rotation via lumberjack) and development.
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util"
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := cfg["filename"].(string); ok && v != "" && v != s.filename {
		s.filename = v
		// Force a reopen at the new path on the next Write. Previously the
		// open handle was left pointing at the old file, so a filename
		// change after the first Write was silently ignored.
		if s.file != nil {
			_ = s.file.Close()
			s.file = nil
		}
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
		if err := s.openLocked(); err != nil {
			return err
		}
	}
	if s.rotateBySizeBytes > 0 {
		if fi, err := s.file.Stat(); err == nil && fi.Size() >= s.rotateBySizeBytes {
			_ = s.file.Close()
			// Clear the handle before reopening so a failed reopen leaves
			// the sink in the "needs open" state and the next Write
			// retries — instead of wedging on a closed fd forever.
			s.file = nil
			s.rotateFiles()
			if err := s.openLocked(); err != nil {
				return err
			}
		}
	}
	return util.WriteJSONToWriter(s.file, entry)
}

// openLocked opens s.filename in APPEND mode. It never uses O_TRUNC: the
// old rotation path reopened with O_TRUNC after a rename, so whenever a
// rotation rename had silently failed the live log file was truncated to
// zero bytes — destroying all accumulated entries. With APPEND, a failed
// rename simply means we keep appending to the existing file.
func (s *OptimalFileSink) openLocked() error {
	dir := filepath.Dir(s.filename)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(s.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	s.file = f
	return nil
}

// Close releases the underlying file handle.
func (s *OptimalFileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		return err
	}
	return nil
}

func (s *OptimalFileSink) rotateFiles() {
	for i := s.maxBackups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", s.filename, i), fmt.Sprintf("%s.%d", s.filename, i+1))
	}
	_ = os.Rename(s.filename, fmt.Sprintf("%s.%d", s.filename, 1))
}

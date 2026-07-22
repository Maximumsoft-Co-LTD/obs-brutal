package file

import (
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

type LumberjackSink struct {
	port.SinkBase
	lj                                *lumberjack.Logger
	mu                                sync.Mutex
	filename                          string
	maxSizeMB, maxBackups, maxAgeDays int
	compress                          bool
}

func NewLumberjackSink() port.Sink {
	return &LumberjackSink{filename: "logs/app.log", maxSizeMB: 50, maxBackups: 7, maxAgeDays: 7, compress: true}
}
func (s *LumberjackSink) ensure() {
	if s.lj != nil {
		return
	}
	s.lj = &lumberjack.Logger{Filename: s.filename, MaxSize: s.maxSizeMB, MaxBackups: s.maxBackups, MaxAge: s.maxAgeDays, Compress: s.compress}
}
func (s *LumberjackSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	// Capture the logger under the lock. Previously Write dereferenced
	// s.lj outside the mutex while Configure set s.lj = nil under it, so
	// a concurrent reconfigure could nil-panic the write and the field
	// access raced. lumberjack.Logger.Write is itself goroutine-safe, so
	// only the pointer read needs the lock.
	s.mu.Lock()
	s.ensure()
	lj := s.lj
	s.mu.Unlock()
	b, err := util.EncodeEntryToJSON(entry)
	if err != nil {
		return err
	}
	_, err = lj.Write(b)
	return err
}

// Close releases the underlying rotating file handle.
func (s *LumberjackSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lj != nil {
		err := s.lj.Close()
		s.lj = nil
		return err
	}
	return nil
}
func (s *LumberjackSink) Name() string { return "lumberjack" }
func (s *LumberjackSink) Configure(cfg map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := cfg["filename"].(string); ok && v != "" {
		s.filename = v
	}
	if v, ok := cfg["max_size_mb"].(int); ok && v > 0 {
		s.maxSizeMB = v
	}
	if v, ok := cfg["max_backups"].(int); ok && v >= 0 {
		s.maxBackups = v
	}
	if v, ok := cfg["max_age_days"].(int); ok && v >= 0 {
		s.maxAgeDays = v
	}
	if v, ok := cfg["compress"].(bool); ok {
		s.compress = v
	}
	// Drop the old logger so the next Write rebuilds with the new config.
	// Close it first so its file handle isn't leaked on reconfigure.
	if s.lj != nil {
		_ = s.lj.Close()
		s.lj = nil
	}
	return nil
}

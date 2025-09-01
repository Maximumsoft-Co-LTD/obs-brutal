package file

import (
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port"
	"obs-brutal/internal/util"
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
	s.mu.Lock()
	s.ensure()
	s.mu.Unlock()
	b, err := util.EncodeEntryToJSON(entry)
	if err != nil {
		return err
	}
	_, err = s.lj.Write(b)
	return err
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
	s.lj = nil
	return nil
}

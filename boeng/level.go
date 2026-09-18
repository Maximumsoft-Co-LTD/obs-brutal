package boeng

import (
	"sync"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/logtrc"
)

// Level is the log severity level. Levels lower than the configured
// Config.Level are dropped before reaching the pipeline.
type Level = logtrc.Level

// Severity levels. Use these constants to set Config.Level.
const (
	DebugLevel = logtrc.DEBUG
	InfoLevel  = logtrc.INFO
	WarnLevel  = logtrc.WARN
	ErrorLevel = logtrc.ERROR
	FatalLevel = logtrc.FATAL
)

// Per-line level policy configured by Init (Config.QuietOps /
// Config.EmitLevel). Read on every op completion and every Emit, written
// only by Init — guarded by levelsMu so hot-reload Init stays race-free.
var (
	levelsMu  sync.RWMutex
	quietOps  bool
	emitLevel = InfoLevel
)

func configureLevels(cfg Config) {
	levelsMu.Lock()
	defer levelsMu.Unlock()
	quietOps = cfg.QuietOps
	emitLevel = InfoLevel
	if cfg.EmitLevel > InfoLevel {
		emitLevel = cfg.EmitLevel
	}
}

func currentLevels() (quiet bool, emit Level) {
	levelsMu.RLock()
	defer levelsMu.RUnlock()
	return quietOps, emitLevel
}

// logAt writes msg on l at the given level. The fluent logger exposes one
// method per level; this is the switch that lets a configured Level pick
// the method.
func logAt(l logtrc.LogBrt, level Level, msg string) {
	switch level {
	case DebugLevel:
		l.Debug(msg)
	case WarnLevel:
		l.Warn(msg)
	case ErrorLevel:
		l.Error(msg)
	case FatalLevel:
		l.Fatal(msg)
	default:
		l.Info(msg)
	}
}

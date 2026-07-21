package boeng

import "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/logtrc"

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

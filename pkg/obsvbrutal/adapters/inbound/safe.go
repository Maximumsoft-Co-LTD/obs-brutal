package inbound

import (
	"context"
	"fmt"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/inbound"
	"runtime/debug"
)

// SafeLoggerAdapter wraps a logger to prevent panics
type SafeLoggerAdapter struct {
	logger inbound.Logger
}

// NewSafeLoggerAdapter creates a new safe logger
func NewSafeLoggerAdapter(logger inbound.Logger) inbound.Logger {
	if logger == nil {
		// Return a no-op logger if nil
		return &noOpLogger{}
	}
	return &SafeLoggerAdapter{logger: logger}
}

// safeCall executes a function and recovers from panics
func (s *SafeLoggerAdapter) safeCall(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't propagate it
			fmt.Printf("Logger panic recovered: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
		}
	}()
	fn()
}

// safeCallWithReturn executes a function that returns a Logger and recovers from panics
func (s *SafeLoggerAdapter) safeCallWithReturn(fn func() inbound.Logger) inbound.Logger {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't propagate it
			fmt.Printf("Logger panic recovered: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
		}
	}()

	result := fn()
	if result == nil {
		return s
	}
	// Wrap the result in another safe logger
	return NewSafeLoggerAdapter(result)
}

// Ctx returns logger with context
func (s *SafeLoggerAdapter) Ctx(ctx context.Context) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.Ctx(ctx)
	})
}

// F returns logger with field
func (s *SafeLoggerAdapter) F(key string, value interface{}) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.F(key, value)
	})
}

// Fs returns logger with fields
func (s *SafeLoggerAdapter) Fs(fields map[string]interface{}) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.Fs(fields)
	})
}

// Err returns logger with error
func (s *SafeLoggerAdapter) Err(err error) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.Err(err)
	})
}

// TID returns logger with trace ID
func (s *SafeLoggerAdapter) TID(traceID string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.TID(traceID)
	})
}

// SID returns logger with span ID
func (s *SafeLoggerAdapter) SID(spanID string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.SID(spanID)
	})
}

// UID returns logger with user ID
func (s *SafeLoggerAdapter) UID(userID string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.UID(userID)
	})
}

// RID returns logger with request ID
func (s *SafeLoggerAdapter) RID(requestID string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.RID(requestID)
	})
}

// IP returns logger with client IP
func (s *SafeLoggerAdapter) IP(ip string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.IP(ip)
	})
}

// Sess returns logger with session ID
func (s *SafeLoggerAdapter) Sess(sessionID string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.Sess(sessionID)
	})
}

// Tenant returns logger with tenant
func (s *SafeLoggerAdapter) Tenant(tenant string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.Tenant(tenant)
	})
}

// Mod returns logger with module
func (s *SafeLoggerAdapter) Mod(module string) inbound.Logger {
	return s.safeCallWithReturn(func() inbound.Logger {
		return s.logger.Mod(module)
	})
}

// Debug logs debug message
func (s *SafeLoggerAdapter) Debug(msg string, fields ...domain.Field) {
	s.safeCall(func() {
		s.logger.Debug(msg, fields...)
	})
}

// Info logs info message
func (s *SafeLoggerAdapter) Info(msg string, fields ...domain.Field) {
	s.safeCall(func() {
		s.logger.Info(msg, fields...)
	})
}

// Warn logs warning message
func (s *SafeLoggerAdapter) Warn(msg string, fields ...domain.Field) {
	s.safeCall(func() {
		s.logger.Warn(msg, fields...)
	})
}

// Error logs error message
func (s *SafeLoggerAdapter) Error(msg string, fields ...domain.Field) {
	s.safeCall(func() {
		s.logger.Error(msg, fields...)
	})
}

// Fatal logs fatal message
func (s *SafeLoggerAdapter) Fatal(msg string, fields ...domain.Field) {
	s.safeCall(func() {
		s.logger.Fatal(msg, fields...)
	})
}

// Level sets log level
func (s *SafeLoggerAdapter) Level(level domain.Level) {
	s.safeCall(func() {
		s.logger.Level(level)
	})
}

// GetLevel returns current log level
func (s *SafeLoggerAdapter) GetLevel() domain.Level {
	var level domain.Level = domain.InfoLevel
	s.safeCall(func() {
		level = s.logger.GetLevel()
	})
	return level
}

// Logged returns logged count
func (s *SafeLoggerAdapter) Logged() int64 {
	var count int64
	s.safeCall(func() {
		count = s.logger.Logged()
	})
	return count
}

// Filtered returns filtered count
func (s *SafeLoggerAdapter) Filtered() int64 {
	var count int64
	s.safeCall(func() {
		count = s.logger.Filtered()
	})
	return count
}

// noOpLogger is a logger that does nothing
type noOpLogger struct{}

func (n *noOpLogger) Ctx(ctx context.Context) inbound.Logger          { return n }
func (n *noOpLogger) F(key string, value interface{}) inbound.Logger  { return n }
func (n *noOpLogger) Fs(fields map[string]interface{}) inbound.Logger { return n }
func (n *noOpLogger) Err(err error) inbound.Logger                    { return n }
func (n *noOpLogger) TID(traceID string) inbound.Logger               { return n }
func (n *noOpLogger) SID(spanID string) inbound.Logger                { return n }
func (n *noOpLogger) UID(userID string) inbound.Logger                { return n }
func (n *noOpLogger) RID(requestID string) inbound.Logger             { return n }
func (n *noOpLogger) IP(ip string) inbound.Logger                     { return n }
func (n *noOpLogger) Sess(sessionID string) inbound.Logger            { return n }
func (n *noOpLogger) Tenant(tenant string) inbound.Logger             { return n }
func (n *noOpLogger) Mod(module string) inbound.Logger                { return n }
func (n *noOpLogger) Debug(msg string, fields ...domain.Field)        {}
func (n *noOpLogger) Info(msg string, fields ...domain.Field)         {}
func (n *noOpLogger) Warn(msg string, fields ...domain.Field)         {}
func (n *noOpLogger) Error(msg string, fields ...domain.Field)        {}
func (n *noOpLogger) Fatal(msg string, fields ...domain.Field)        {}
func (n *noOpLogger) Level(level domain.Level)                        {}
func (n *noOpLogger) GetLevel() domain.Level                          { return domain.InfoLevel }
func (n *noOpLogger) Logged() int64                                   { return 0 }
func (n *noOpLogger) Filtered() int64                                 { return 0 }

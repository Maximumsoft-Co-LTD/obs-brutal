package shared

import (
	bufSink "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/sink/buffered"
	fileSink "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/sink/file"
	netSink "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/sink/network"
	stdout "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/sink/stdout"
	zlogSink "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/outbound/sink/zerolog"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"time"
)

// SinkOption builds a configuration map to pass into sink.Configure
type SinkOption func(cfg map[string]interface{})

func applyOptions(s port.Sink, opts ...SinkOption) port.Sink {
	if len(opts) == 0 || s == nil {
		return s
	}
	cfg := make(map[string]interface{})
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	_ = s.Configure(cfg)
	return s
}

// Core sinks
func (f *Factory) FastStdout(opts ...SinkOption) port.Sink {
	return applyOptions(stdout.NewFastStdoutSink(), opts...)
}
func (f *Factory) JSON(opts ...SinkOption) port.Sink {
	return applyOptions(stdout.NewJSONSink(), opts...)
}
func (f *Factory) Toggle(inner port.Sink, enabled bool, opts ...SinkOption) port.Sink {
	return applyOptions(stdout.NewToggleSink(inner, enabled), opts...)
}
func (f *Factory) Buffered(opts ...SinkOption) port.Sink {
	return applyOptions(bufSink.NewBufferedSink(), opts...)
}
func (f *Factory) BufferedWith(size int, timeout time.Duration, opts ...SinkOption) port.Sink {
	return applyOptions(bufSink.NewBufferedSinkWith(size, timeout), opts...)
}

// BufferedWrap wraps an inner sink with a buffer.
func (f *Factory) BufferedWrap(inner port.Sink, size int, timeout time.Duration, opts ...SinkOption) port.Sink {
	return applyOptions(bufSink.NewBuf(inner, size, timeout), opts...)
}

// File sinks
func (f *Factory) File(opts ...SinkOption) port.Sink {
	return applyOptions(fileSink.NewOptimalFileSink(), opts...)
}
func (f *Factory) Lumberjack(opts ...SinkOption) port.Sink {
	return applyOptions(fileSink.NewLumberjackSink(), opts...)
}

// Network sinks
func (f *Factory) Loki(endpoint string, labels map[string]string, opts ...SinkOption) port.Sink {
	return applyOptions(netSink.NewLokiPushSink(endpoint, labels), opts...)
}
func (f *Factory) ClickHouse(opts ...SinkOption) port.Sink {
	return applyOptions(netSink.NewClickHouseSink(), opts...)
}
func (f *Factory) Zerolog(opts ...SinkOption) port.Sink {
	return applyOptions(zlogSink.NewZerologSink(), opts...)
}

// Option helpers (common keys)
func WithEnabled(v bool) SinkOption { return func(cfg map[string]interface{}) { cfg["enabled"] = v } }
func WithFilename(path string) SinkOption {
	return func(cfg map[string]interface{}) { cfg["filename"] = path }
}
func WithRotateSizeBytes(n int64) SinkOption {
	return func(cfg map[string]interface{}) { cfg["rotate_size_bytes"] = n }
}
func WithMaxBackups(n int) SinkOption {
	return func(cfg map[string]interface{}) { cfg["max_backups"] = n }
}
func WithMaxSizeMB(n int) SinkOption {
	return func(cfg map[string]interface{}) { cfg["max_size_mb"] = n }
}
func WithMaxAgeDays(n int) SinkOption {
	return func(cfg map[string]interface{}) { cfg["max_age_days"] = n }
}
func WithCompress(v bool) SinkOption { return func(cfg map[string]interface{}) { cfg["compress"] = v } }
func WithEndpoint(url string) SinkOption {
	return func(cfg map[string]interface{}) { cfg["endpoint"] = url }
}
func WithLabels(labels map[string]string) SinkOption {
	return func(cfg map[string]interface{}) { cfg["labels"] = labels }
}
func WithUsername(u string) SinkOption {
	return func(cfg map[string]interface{}) { cfg["username"] = u }
}
func WithPassword(p string) SinkOption {
	return func(cfg map[string]interface{}) { cfg["password"] = p }
}
func WithAutoCreate(v bool) SinkOption {
	return func(cfg map[string]interface{}) { cfg["auto_create"] = v }
}

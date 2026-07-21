package port

import "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"

// FilterStrategy decides whether a log should be written
type FilterStrategy interface {
	ShouldLog(entry *domain.LogEntry) bool
	Name() string
	Configure(config map[string]interface{}) error
}

// SamplerStrategy decides whether to sample a log entry
type SamplerStrategy interface {
	ShouldSample(entry *domain.LogEntry) bool
	Name() string
	Configure(cfg map[string]interface{}) error
}

// MaskingStrategy masks fields in a log entry
type MaskingStrategy interface {
	MaskFields(fields map[string]interface{}) map[string]interface{}
	Name() string
	Configure(cfg map[string]interface{}) error
}

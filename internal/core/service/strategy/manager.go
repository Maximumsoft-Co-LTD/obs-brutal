package strategy

import (
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	"sync"
)

// Manager holds configured strategies and applies them in order.
type Manager struct {
	filters  []port.FilterStrategy
	samplers []port.SamplerStrategy
	maskers  []port.MaskingStrategy
	mu       sync.RWMutex
}

// NewManager constructs an empty strategy manager.
func NewManager() *Manager {
	return &Manager{filters: []port.FilterStrategy{}, samplers: []port.SamplerStrategy{}, maskers: []port.MaskingStrategy{}}
}

// AddFilter appends a filter to the pipeline.
func (m *Manager) AddFilter(f port.FilterStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filters = append(m.filters, f)
}

// AddSampler appends a sampler to the pipeline.
func (m *Manager) AddSampler(s port.SamplerStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.samplers = append(m.samplers, s)
}

// AddMasker appends a masker to the pipeline.
func (m *Manager) AddMasker(ms port.MaskingStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maskers = append(m.maskers, ms)
}

// RemoveStrategy removes a strategy by kind (filter|sampler|masker) and name.
func (m *Manager) RemoveStrategy(kind, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch kind {
	case "filter":
		nf := make([]port.FilterStrategy, 0, len(m.filters))
		for _, f := range m.filters {
			if f.Name() != name {
				nf = append(nf, f)
			}
		}
		m.filters = nf
	case "sampler":
		ns := make([]port.SamplerStrategy, 0, len(m.samplers))
		for _, s := range m.samplers {
			if s.Name() != name {
				ns = append(ns, s)
			}
		}
		m.samplers = ns
	case "masker":
		nm := make([]port.MaskingStrategy, 0, len(m.maskers))
		for _, ms := range m.maskers {
			if ms.Name() != name {
				nm = append(nm, ms)
			}
		}
		m.maskers = nm
	}
}

// ProcessEntry runs filters, samplers and maskers in order.
// Returns the (possibly masked) entry, or nil if filtered/sampled out.
func (m *Manager) ProcessEntry(entry *domain.LogEntry) *domain.LogEntry {
	if entry == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, f := range m.filters {
		if !f.ShouldLog(entry) {
			return nil
		}
	}
	for _, s := range m.samplers {
		if !s.ShouldSample(entry) {
			return nil
		}
	}
	if len(m.maskers) == 0 {
		return entry
	}
	masked := &domain.LogEntry{Level: entry.Level, Msg: entry.Msg, Timestamp: entry.Timestamp, Mod: entry.Mod, TenantID: entry.TenantID, UserID: entry.UserID, TraceID: entry.TraceID, SpanID: entry.SpanID, RequestID: entry.RequestID, Err: entry.Err, F: make(map[string]interface{}, len(entry.F))}
	for k, v := range entry.F {
		masked.F[k] = v
	}
	for _, ms := range m.maskers {
		masked.F = ms.MaskFields(masked.F)
	}
	return masked
}

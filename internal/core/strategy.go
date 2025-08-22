// Package core provides Strategy Pattern for dynamic logging behavior
package core

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"obs-brutal/internal/core/domain"
)

// ===== STRATEGY INTERFACES =====

// FilterStrategy determines if a log entry should be processed
type FilterStrategy interface {
	ShouldLog(entry *domain.LogEntry) bool
	Name() string
	Configure(config map[string]interface{}) error
}

// SamplerStrategy determines sampling behavior
type SamplerStrategy interface {
	ShouldSample(entry *domain.LogEntry) bool
	Name() string
	Configure(config map[string]interface{}) error
}

// MaskingStrategy handles sensitive data masking
type MaskingStrategy interface {
	MaskFields(fields map[string]interface{}) map[string]interface{}
	Name() string
	Configure(config map[string]interface{}) error
}

// ===== FILTER STRATEGIES =====

// LevelFilter filters by log level
type LevelFilter struct {
	minLevel domain.Level
	maxLevel domain.Level
}

func NewLevelFilter(minLevel, maxLevel domain.Level) *LevelFilter {
	return &LevelFilter{minLevel: minLevel, maxLevel: maxLevel}
}

func (f *LevelFilter) ShouldLog(entry *domain.LogEntry) bool {
	return entry.Level >= f.minLevel && entry.Level <= f.maxLevel
}

func (f *LevelFilter) Name() string { return "level_filter" }

func (f *LevelFilter) Configure(config map[string]interface{}) error {
	if min, ok := config["min_level"].(domain.Level); ok {
		f.minLevel = min
	}
	if max, ok := config["max_level"].(domain.Level); ok {
		f.maxLevel = max
	}
	return nil
}

// ModuleFilter filters by module name
type ModuleFilter struct {
	allowedModules map[string]bool
	blockedModules map[string]bool
	mu             sync.RWMutex
}

func NewModuleFilter() *ModuleFilter {
	return &ModuleFilter{
		allowedModules: make(map[string]bool),
		blockedModules: make(map[string]bool),
	}
}

func (f *ModuleFilter) AllowModule(module string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowedModules[module] = true
}

func (f *ModuleFilter) BlockModule(module string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blockedModules[module] = true
}

func (f *ModuleFilter) ShouldLog(entry *domain.LogEntry) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	module := entry.Module
	if module == "" {
		if mod, ok := entry.Fields["module"].(string); ok {
			module = mod
		}
	}

	// Check blocked first
	if f.blockedModules[module] {
		return false
	}

	// If allowed list exists, check it
	if len(f.allowedModules) > 0 {
		return f.allowedModules[module]
	}

	// Allow by default
	return true
}

func (f *ModuleFilter) Name() string { return "module_filter" }

func (f *ModuleFilter) Configure(config map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Go 1.25: Use clear() for efficient map reset
	clear(f.allowedModules)
	clear(f.blockedModules)

	if allowed, ok := config["allowed"].([]string); ok {
		for _, module := range allowed {
			f.allowedModules[module] = true
		}
	}

	if blocked, ok := config["blocked"].([]string); ok {
		for _, module := range blocked {
			f.blockedModules[module] = true
		}
	}

	return nil
}

// ===== SAMPLER STRATEGIES =====

// RateSampler samples based on rate limit
type RateSampler struct {
	rate     float64 // 0.0 to 1.0
	counter  atomic.Uint64
	interval uint64
}

func NewRateSampler(rate float64) *RateSampler {
	// Clamp and compute interval safely
	if rate >= 1.0 {
		return &RateSampler{rate: 1.0, interval: 1}
	}
	if rate <= 0.0 {
		return &RateSampler{rate: 0.0, interval: 0}
	}
	interval := uint64(math.Ceil(1.0 / rate))
	if interval == 0 {
		interval = 1
	}
	return &RateSampler{rate: rate, interval: interval}
}

func (s *RateSampler) ShouldSample(entry *domain.LogEntry) bool {
	if s.rate <= 0.0 {
		return false
	}
	if s.rate >= 1.0 {
		return true
	}
	if s.interval == 0 {
		return false
	}
	count := s.counter.Add(1)
	return count%s.interval == 0
}

func (s *RateSampler) Name() string { return "rate_sampler" }

func (s *RateSampler) Configure(config map[string]interface{}) error {
	if rate, ok := config["rate"].(float64); ok {
		// Clamp and compute safely
		if rate >= 1.0 {
			s.rate = 1.0
			s.interval = 1
		} else if rate <= 0.0 {
			s.rate = 0.0
			s.interval = 0
		} else {
			s.rate = rate
			interval := uint64(math.Ceil(1.0 / rate))
			if interval == 0 {
				interval = 1
			}
			s.interval = interval
		}
	}
	return nil
}

// AdaptiveSampler adjusts sampling based on load
type AdaptiveSampler struct {
	baseRate    float64
	currentRate atomic.Uint64 // stored as uint64 * 1000000 for precision
	maxRate     float64
	minRate     float64

	// Load tracking
	logCount       atomic.Uint64
	lastAdjust     atomic.Int64 // Unix timestamp
	adjustInterval time.Duration
}

func NewAdaptiveSampler(baseRate, minRate, maxRate float64) *AdaptiveSampler {
	sampler := &AdaptiveSampler{
		baseRate:       baseRate,
		maxRate:        maxRate,
		minRate:        minRate,
		adjustInterval: 10 * time.Second,
	}

	sampler.currentRate.Store(uint64(baseRate * 1000000))
	sampler.lastAdjust.Store(time.Now().Unix())

	return sampler
}

func (s *AdaptiveSampler) ShouldSample(entry *domain.LogEntry) bool {
	// Adjust rate periodically
	now := time.Now().Unix()
	lastAdjust := s.lastAdjust.Load()

	if now-lastAdjust > int64(s.adjustInterval.Seconds()) {
		s.adjustSamplingRate()
		s.lastAdjust.Store(now)
	}

	// Sample based on current rate
	currentRate := float64(s.currentRate.Load()) / 1000000
	if currentRate <= 0.0 {
		return false
	}
	if currentRate >= 1.0 {
		return true
	}
	interval := uint64(math.Ceil(1.0 / currentRate))
	if interval == 0 {
		interval = 1
	}
	count := s.logCount.Add(1)
	return count%interval == 0
}

func (s *AdaptiveSampler) adjustSamplingRate() {
	// Simple load-based adjustment
	currentCount := s.logCount.Load()

	// High load - reduce sampling
	if currentCount > 10000 {
		newRate := s.getCurrentRate() * 0.8
		if newRate < s.minRate {
			newRate = s.minRate
		}
		s.setCurrentRate(newRate)
	} else if currentCount < 1000 {
		// Low load - increase sampling
		newRate := s.getCurrentRate() * 1.2
		if newRate > s.maxRate {
			newRate = s.maxRate
		}
		s.setCurrentRate(newRate)
	}

	// Reset counter
	s.logCount.Store(0)
}

func (s *AdaptiveSampler) getCurrentRate() float64 {
	return float64(s.currentRate.Load()) / 1000000
}

func (s *AdaptiveSampler) setCurrentRate(rate float64) {
	s.currentRate.Store(uint64(rate * 1000000))
}

func (s *AdaptiveSampler) Name() string { return "adaptive_sampler" }

func (s *AdaptiveSampler) Configure(config map[string]interface{}) error {
	if rate, ok := config["base_rate"].(float64); ok {
		s.baseRate = rate
		s.setCurrentRate(rate)
	}
	return nil
}

// ===== MASKING STRATEGIES =====

// RegexMaskingStrategy uses Go 1.25 enhanced regex for PII masking
type RegexMaskingStrategy struct {
	patterns map[string]*regexp.Regexp
	mu       sync.RWMutex
}

func NewRegexMaskingStrategy() *RegexMaskingStrategy {
	strategy := &RegexMaskingStrategy{
		patterns: make(map[string]*regexp.Regexp),
	}

	// Default patterns for common PII
	strategy.addDefaultPatterns()

	return strategy
}

func (m *RegexMaskingStrategy) addDefaultPatterns() {
	patterns := map[string]string{
		// Thai ID Card: 1-3456-78901-23-4
		"thai_id": `\b\d{1}-\d{4}-\d{5}-\d{2}-\d{1}\b`,
		// Phone: 08x-xxx-xxxx or +66-x-xxx-xxxx
		"phone": `(\+66|0)[0-9-]{10,13}`,
		// Email: xxx@domain.com
		"email": `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		// Credit Card: xxxx-xxxx-xxxx-xxxx
		"credit_card": `\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`,
		// Password patterns
		"password": `(?i)(password|passwd|pwd)["'\s]*[:=]["'\s]*[^"',\s]+`,
	}

	for name, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			m.patterns[name] = compiled
		}
	}
}

func (m *RegexMaskingStrategy) MaskFields(fields map[string]interface{}) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Go 1.25: Efficient map iteration and modification
	masked := make(map[string]interface{}, len(fields))

	for k, v := range fields {
		if str, ok := v.(string); ok {
			masked[k] = m.maskString(str)
		} else {
			masked[k] = v
		}
	}

	return masked
}

func (m *RegexMaskingStrategy) maskString(input string) string {
	result := input

	for name, pattern := range m.patterns {
		switch name {
		case "thai_id":
			result = pattern.ReplaceAllString(result, "x-xxxx-xxxxx-xx-x")
		case "phone":
			result = pattern.ReplaceAllString(result, "xxx-xxx-xxxx")
		case "email":
			result = pattern.ReplaceAllStringFunc(result, func(match string) string {
				parts := strings.Split(match, "@")
				if len(parts) == 2 {
					return "***@" + parts[1]
				}
				return "***"
			})
		case "credit_card":
			result = pattern.ReplaceAllString(result, "****-****-****-****")
		case "password":
			result = pattern.ReplaceAllString(result, "$1: ***")
		}
	}

	return result
}

func (m *RegexMaskingStrategy) Name() string { return "regex_masking" }

func (m *RegexMaskingStrategy) Configure(config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if patterns, ok := config["patterns"].(map[string]string); ok {
		// Go 1.25: Clear existing patterns efficiently
		clear(m.patterns)

		for name, pattern := range patterns {
			if compiled, err := regexp.Compile(pattern); err == nil {
				m.patterns[name] = compiled
			}
		}
	}

	return nil
}

// ===== STRATEGY MANAGER =====

// StrategyManager manages all logging strategies
type StrategyManager struct {
	filters  []FilterStrategy
	samplers []SamplerStrategy
	maskers  []MaskingStrategy
	mu       sync.RWMutex
}

func NewStrategyManager() *StrategyManager {
	return &StrategyManager{
		filters:  make([]FilterStrategy, 0),
		samplers: make([]SamplerStrategy, 0),
		maskers:  make([]MaskingStrategy, 0),
	}
}

// AddFilter adds a filter strategy
func (sm *StrategyManager) AddFilter(filter FilterStrategy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.filters = append(sm.filters, filter)
}

// AddSampler adds a sampler strategy
func (sm *StrategyManager) AddSampler(sampler SamplerStrategy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.samplers = append(sm.samplers, sampler)
}

// AddMasker adds a masking strategy
func (sm *StrategyManager) AddMasker(masker MaskingStrategy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.maskers = append(sm.maskers, masker)
}

// ProcessEntry applies all strategies to a log entry
func (sm *StrategyManager) ProcessEntry(entry *domain.LogEntry) *domain.LogEntry {
	if entry == nil {
		return nil
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Apply filters first
	for _, filter := range sm.filters {
		if !filter.ShouldLog(entry) {
			return nil // Filtered out
		}
	}

	// Apply sampling
	for _, sampler := range sm.samplers {
		if !sampler.ShouldSample(entry) {
			return nil // Sampled out
		}
	}

	// Apply masking
	if len(sm.maskers) > 0 {
		// Create copy to avoid modifying original
		maskedEntry := &domain.LogEntry{
			Level:     entry.Level,
			Message:   entry.Message,
			Timestamp: entry.Timestamp,
			Module:    entry.Module,
			TenantID:  entry.TenantID,
			UserID:    entry.UserID,
			TraceID:   entry.TraceID,
			SpanID:    entry.SpanID,
			RequestID: entry.RequestID,
			Error:     entry.Error,
			Fields:    make(map[string]interface{}, len(entry.Fields)),
		}

		// Copy and mask fields
		for k, v := range entry.Fields {
			maskedEntry.Fields[k] = v
		}

		// Apply all masking strategies
		for _, masker := range sm.maskers {
			maskedEntry.Fields = masker.MaskFields(maskedEntry.Fields)
		}

		return maskedEntry
	}

	return entry
}

// RemoveStrategy removes strategy by name
func (sm *StrategyManager) RemoveStrategy(strategyType, name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch strategyType {
	case "filter":
		// Go 1.25: Efficient slice operations
		newFilters := make([]FilterStrategy, 0, len(sm.filters))
		for _, filter := range sm.filters {
			if filter.Name() != name {
				newFilters = append(newFilters, filter)
			}
		}
		sm.filters = newFilters

	case "sampler":
		newSamplers := make([]SamplerStrategy, 0, len(sm.samplers))
		for _, sampler := range sm.samplers {
			if sampler.Name() != name {
				newSamplers = append(newSamplers, sampler)
			}
		}
		sm.samplers = newSamplers

	case "masker":
		newMaskers := make([]MaskingStrategy, 0, len(sm.maskers))
		for _, masker := range sm.maskers {
			if masker.Name() != name {
				newMaskers = append(newMaskers, masker)
			}
		}
		sm.maskers = newMaskers
	}
}

// ===== ENHANCED LogBrt WITH STRATEGIES =====

// StrategyLogBrt combines unified LogBrt with strategies
type StrategyLogBrt struct {
	*UnifiedLogBrt
	strategies *StrategyManager
	async      *AsyncPipeline
}

// NewStrategyLogBrt creates LogBrt with strategy support
func NewStrategyLogBrt(level Level, sinks ...Sink) *StrategyLogBrt {
	strategies := NewStrategyManager()

	// Add default strategies
	strategies.AddMasker(NewRegexMaskingStrategy())
	strategies.AddSampler(NewRateSampler(1.0)) // 100% by default

	// Create async pipeline for maximum performance
	async := NewAsyncPipeline(1000, 4, 100*time.Millisecond, sinks...)

	baseLogBrt := NewUnifiedLogBrt(level)

	return &StrategyLogBrt{
		UnifiedLogBrt: baseLogBrt,
		strategies:    strategies,
		async:         async,
	}
}

// Override log method to apply strategies
func (sl *StrategyLogBrt) log(level domain.Level, msg string) {
	// Fast level check
	if level < sl.level {
		return
	}

	// Create log entry
	entry := newLogEntry(level, msg, sl.fields)

	// Apply strategies (filter, sample, mask)
	processedEntry := sl.strategies.ProcessEntry(entry)
	if processedEntry == nil {
		return // Filtered or sampled out
	}

	// Write async for maximum performance
	if !sl.async.WriteAsync(processedEntry) {
		// Fallback to sync if pipeline is full
		for _, sink := range sl.async.sinks {
			if sink != nil {
				sink.Write(processedEntry)
			}
		}
	}

	// Update counter
	if sl.logCount != nil {
		sl.logCount.Add(1)
	}
}

// ===== OVERRIDES: ensure StrategyLogBrt methods dispatch to its own log =====
func (sl *StrategyLogBrt) Debug(msg string) { sl.log(domain.DebugLevel, msg) }
func (sl *StrategyLogBrt) Info(msg string)  { sl.log(domain.InfoLevel, msg) }
func (sl *StrategyLogBrt) Warn(msg string)  { sl.log(domain.WarnLevel, msg) }
func (sl *StrategyLogBrt) Error(msg string) { sl.log(domain.ErrorLevel, msg) }
func (sl *StrategyLogBrt) Fatal(msg string) { sl.log(domain.FatalLevel, msg) }

func (sl *StrategyLogBrt) Debugf(format string, args ...interface{}) {
	sl.log(domain.DebugLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Infof(format string, args ...interface{}) {
	sl.log(domain.InfoLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Warnf(format string, args ...interface{}) {
	sl.log(domain.WarnLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Errorf(format string, args ...interface{}) {
	sl.log(domain.ErrorLevel, fmt.Sprintf(format, args...))
}
func (sl *StrategyLogBrt) Fatalf(format string, args ...interface{}) {
	sl.log(domain.FatalLevel, fmt.Sprintf(format, args...))
}

// ===== DERIVED BUILDER OVERRIDES: preserve StrategyLogBrt type =====

// clone creates a StrategyLogBrt that preserves strategies/async while cloning base fields safely
func (sl *StrategyLogBrt) clone() *StrategyLogBrt {
	baseClone := sl.UnifiedLogBrt.clone()
	return &StrategyLogBrt{
		UnifiedLogBrt: baseClone,
		strategies:    sl.strategies,
		async:         sl.async,
	}
}

// F adds a field and returns StrategyLogBrt for continued strategy/async behavior
func (sl *StrategyLogBrt) F(key string, value interface{}) LogBrt {
	clone := sl.clone()
	clone.fields[key] = value
	return clone
}

// Fs adds multiple fields and returns StrategyLogBrt
func (sl *StrategyLogBrt) Fs(fields map[string]interface{}) LogBrt {
	if len(fields) == 0 {
		return sl
	}
	clone := sl.clone()
	for k, v := range fields {
		clone.fields[k] = v
	}
	return clone
}

// Ctx attaches context and common IDs while preserving type
func (sl *StrategyLogBrt) Ctx(ctx context.Context) LogBrt {
	if ctx == nil {
		return sl
	}
	clone := sl.clone()
	clone.fields["context"] = ctx
	if traceID := extractFromContext(ctx, "trace_id"); traceID != "" {
		clone.fields["trace_id"] = traceID
	}
	if userID := extractFromContext(ctx, "user_id"); userID != "" {
		clone.fields["user_id"] = userID
	}
	if requestID := extractFromContext(ctx, "request_id"); requestID != "" {
		clone.fields["request_id"] = requestID
	}
	return clone
}

func (sl *StrategyLogBrt) TraceID(id string) LogBrt   { return sl.F("trace_id", id) }
func (sl *StrategyLogBrt) UserID(id string) LogBrt    { return sl.F("user_id", id) }
func (sl *StrategyLogBrt) RequestID(id string) LogBrt { return sl.F("request_id", id) }

// WithError attaches error while preserving type
func (sl *StrategyLogBrt) WithError(err error) LogBrt {
	if err == nil {
		return sl
	}
	return sl.F("error", err.Error())
}

// Strategy management methods
func (sl *StrategyLogBrt) AddFilter(filter FilterStrategy) {
	sl.strategies.AddFilter(filter)
}

func (sl *StrategyLogBrt) AddSampler(sampler SamplerStrategy) {
	sl.strategies.AddSampler(sampler)
}

func (sl *StrategyLogBrt) AddMasker(masker MaskingStrategy) {
	sl.strategies.AddMasker(masker)
}

func (sl *StrategyLogBrt) RemoveStrategy(strategyType, name string) {
	sl.strategies.RemoveStrategy(strategyType, name)
}

// GetStats returns comprehensive statistics
func (sl *StrategyLogBrt) GetStats() StrategyStats {
	asyncStats := sl.async.Stats()

	return StrategyStats{
		LogsProcessed: sl.LogCount(),
		AsyncStats:    asyncStats,
		Strategies: StrategyInfo{
			FiltersCount:  len(sl.strategies.filters),
			SamplersCount: len(sl.strategies.samplers),
			MaskersCount:  len(sl.strategies.maskers),
		},
	}
}

// Stop gracefully stops all async operations
func (sl *StrategyLogBrt) Stop() {
	sl.async.Stop()
}

// StrategyStats holds comprehensive statistics
type StrategyStats struct {
	LogsProcessed int64        `json:"logs_processed"`
	AsyncStats    AsyncStats   `json:"async_stats"`
	Strategies    StrategyInfo `json:"strategies"`
}

// StrategyInfo holds strategy information
type StrategyInfo struct {
	FiltersCount  int `json:"filters_count"`
	SamplersCount int `json:"samplers_count"`
	MaskersCount  int `json:"maskers_count"`
}

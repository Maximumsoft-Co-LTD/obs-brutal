package strategy

import (
	"math/rand"
	"regexp"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
)

// LevelFilter filters logs by level range [min,max]
type LevelFilter struct{ min, max domain.Level }

func NewLevelFilter(min, max domain.Level) port.FilterStrategy {
	return &LevelFilter{min: min, max: max}
}
func (lf *LevelFilter) ShouldLog(entry *domain.LogEntry) bool {
	return entry != nil && entry.Level >= lf.min && entry.Level <= lf.max
}
func (lf *LevelFilter) Name() string { return "level_filter" }
func (lf *LevelFilter) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["min"].(domain.Level); ok {
		lf.min = v
	}
	if v, ok := cfg["max"].(domain.Level); ok {
		lf.max = v
	}
	return nil
}

// RateSampler samples logs randomly by rate [0,1]
type RateSampler struct {
	rate float64
	rng  *rand.Rand
}

func NewRateSampler(rate float64) port.SamplerStrategy {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	// Use a private RNG to avoid touching global seed/state
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &RateSampler{rate: rate, rng: rng}
}

// NewRateSamplerWithRNG allows external RNG injection (testing or deterministic behavior)
func NewRateSamplerWithRNG(rate float64, rng *rand.Rand) port.SamplerStrategy {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &RateSampler{rate: rate, rng: rng}
}
func (rs *RateSampler) ShouldSample(_ *domain.LogEntry) bool {
	if rs.rng == nil {
		rs.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return rs.rng.Float64() < rs.rate
}
func (rs *RateSampler) Name() string { return "rate_sampler" }
func (rs *RateSampler) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["rate"].(float64); ok {
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		rs.rate = v
	}
	return nil
}

// AdaptiveSampler a simple wrapper that uses base rate (placeholder)
type AdaptiveSampler struct{ base, min, max float64 }

func NewAdaptiveSampler(baseRate, minRate, maxRate float64) port.SamplerStrategy {
	clamp := func(x float64) float64 {
		if x < 0 {
			return 0
		}
		if x > 1 {
			return 1
		}
		return x
	}
	return &AdaptiveSampler{base: clamp(baseRate), min: clamp(minRate), max: clamp(maxRate)}
}
func (as *AdaptiveSampler) ShouldSample(_ *domain.LogEntry) bool {
	r := as.base
	if r < as.min {
		r = as.min
	}
	if r > as.max {
		r = as.max
	}
	return rand.Float64() < r
}
func (as *AdaptiveSampler) Name() string { return "adaptive_sampler" }
func (as *AdaptiveSampler) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["base"].(float64); ok {
		as.base = v
	}
	if v, ok := cfg["min"].(float64); ok {
		as.min = v
	}
	if v, ok := cfg["max"].(float64); ok {
		as.max = v
	}
	return nil
}

// RegexMaskingStrategy masks sensitive strings using regex patterns
type RegexMaskingStrategy struct{ patterns []*regexp.Regexp }

func NewRegexMaskingStrategy() port.MaskingStrategy {
	pats := []*regexp.Regexp{
		regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),    // email
		regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),                       // credit card
		regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]*`), // JWT
	}
	return &RegexMaskingStrategy{patterns: pats}
}
func (rm *RegexMaskingStrategy) MaskFields(fields map[string]interface{}) map[string]interface{} {
	if len(rm.patterns) == 0 || len(fields) == 0 {
		return fields
	}
	out := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		if s, ok := v.(string); ok {
			masked := s
			for _, re := range rm.patterns {
				masked = re.ReplaceAllString(masked, "***")
			}
			out[k] = masked
		} else {
			out[k] = v
		}
	}
	return out
}
func (rm *RegexMaskingStrategy) Name() string                               { return "regex_masker" }
func (rm *RegexMaskingStrategy) Configure(cfg map[string]interface{}) error { _ = cfg; return nil }

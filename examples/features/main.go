package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"
)

// ตัวอย่างการสร้าง Custom Features และ Error Categories
func main() {
	// สร้าง logger
	logger, err := logbrutal.NewLogger(
		logbrutal.WithLevel(logbrutal.InfoLevel),
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// สร้าง feature registry
	featureRegistry := NewFeatureRegistry()

	// ลงทะเบียน custom features
	registerCustomFeatures(featureRegistry)

	// สร้าง error category registry
	errorRegistry := NewErrorCategoryRegistry()

	// ลงทะเบียน error categories
	registerErrorCategories(errorRegistry)

	// Demo features
	log.Println("\n=== Custom Features Demo ===")
	demoFeatures(logger, featureRegistry)

	// Demo error categories
	log.Println("\n=== Error Categories Demo ===")
	demoErrorCategories(logger, errorRegistry)
}

// === CUSTOM FEATURES ===

// 1. Performance Monitoring Feature
type PerformanceFeature struct {
	threshold time.Duration
}

func (f *PerformanceFeature) Name() string {
	return "performance"
}

func (f *PerformanceFeature) Apply(logger inbound.Logger) inbound.Logger {
	// Add performance tracking to logger
	return logger.F("perf_threshold_ms", f.threshold.Milliseconds())
}

func (f *PerformanceFeature) Configure(config map[string]interface{}) error {
	if ms, ok := config["threshold_ms"].(float64); ok {
		f.threshold = time.Duration(ms) * time.Millisecond
	}
	return nil
}

// 2. Security Audit Feature
type SecurityAuditFeature struct {
	enabled bool
	level   string
}

func (f *SecurityAuditFeature) Name() string {
	return "security_audit"
}

func (f *SecurityAuditFeature) Apply(logger inbound.Logger) inbound.Logger {
	if !f.enabled {
		return logger
	}
	// Add security context
	return logger.
		F("audit_enabled", true).
		F("audit_level", f.level).
		F("audit_timestamp", time.Now().Unix())
}

func (f *SecurityAuditFeature) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		f.enabled = enabled
	}
	if level, ok := config["level"].(string); ok {
		f.level = level
	}
	return nil
}

// 3. Compliance Feature (GDPR, PDPA)
type ComplianceFeature struct {
	regulation string
	maskPII    bool
}

func (f *ComplianceFeature) Name() string {
	return "compliance"
}

func (f *ComplianceFeature) Apply(logger inbound.Logger) inbound.Logger {
	return logger.
		F("compliance", f.regulation).
		F("pii_masked", f.maskPII)
}

func (f *ComplianceFeature) Configure(config map[string]interface{}) error {
	if reg, ok := config["regulation"].(string); ok {
		f.regulation = reg
	}
	if mask, ok := config["mask_pii"].(bool); ok {
		f.maskPII = mask
	}
	return nil
}

// === ERROR CATEGORIES ===

// 1. Validation Error Handler
type ValidationErrorHandler struct{}

func (h *ValidationErrorHandler) Category() string {
	return "validation"
}

func (h *ValidationErrorHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.
		F("error_category", "validation").
		F("error_type", "input_validation").
		Fs(details).
		Err(err).
		Warn("Validation error occurred")
}

func (h *ValidationErrorHandler) ShouldAlert() bool {
	return false // Validation errors don't need alerts
}

func (h *ValidationErrorHandler) Severity() domain.Level {
	return domain.WarnLevel
}

// 2. Database Error Handler
type DatabaseErrorHandler struct {
	alertThreshold int
	errorCount     int
	mu             sync.Mutex
}

func (h *DatabaseErrorHandler) Category() string {
	return "database"
}

func (h *DatabaseErrorHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	h.mu.Lock()
	h.errorCount++
	count := h.errorCount
	h.mu.Unlock()

	logger.
		F("error_category", "database").
		F("error_type", "connection").
		F("error_count", count).
		Fs(details).
		Err(err).
		Error("Database error occurred")
}

func (h *DatabaseErrorHandler) ShouldAlert() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.errorCount >= h.alertThreshold
}

func (h *DatabaseErrorHandler) Severity() domain.Level {
	return domain.ErrorLevel
}

// 3. Payment Error Handler
type PaymentErrorHandler struct{}

func (h *PaymentErrorHandler) Category() string {
	return "payment"
}

func (h *PaymentErrorHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	// Payment errors need detailed logging
	logger.
		F("error_category", "payment").
		F("error_critical", true).
		F("requires_manual_review", true).
		Fs(details).
		Err(err).
		Error("Payment processing error - requires immediate attention")
}

func (h *PaymentErrorHandler) ShouldAlert() bool {
	return true // Always alert on payment errors
}

func (h *PaymentErrorHandler) Severity() domain.Level {
	return domain.ErrorLevel
}

// === DEMO FUNCTIONS ===

func registerCustomFeatures(registry inbound.Features) {
	// Register performance feature
	perfFeature := &PerformanceFeature{threshold: 100 * time.Millisecond}
	perfFeature.Configure(map[string]interface{}{
		"threshold_ms": 200.0,
	})
	registry.Register("performance", perfFeature)

	// Register security audit feature
	secFeature := &SecurityAuditFeature{}
	secFeature.Configure(map[string]interface{}{
		"enabled": true,
		"level":   "high",
	})
	registry.Register("security_audit", secFeature)

	// Register compliance feature
	compFeature := &ComplianceFeature{}
	compFeature.Configure(map[string]interface{}{
		"regulation": "PDPA",
		"mask_pii":   true,
	})
	registry.Register("compliance", compFeature)

	log.Printf("Registered features: %v", registry.List())
}

func registerErrorCategories(registry inbound.ErrCategories) {
	// Register error handlers
	registry.Register("validation", &ValidationErrorHandler{})
	registry.Register("database", &DatabaseErrorHandler{alertThreshold: 5})
	registry.Register("payment", &PaymentErrorHandler{})

	log.Printf("Registered error categories: %v", registry.List())
}

func demoFeatures(logger inbound.Logger, registry inbound.Features) {
	// Apply single feature
	perfFeature, _ := registry.Get("performance")
	perfLogger := perfFeature.Apply(logger)
	perfLogger.Info("Logger with performance tracking")

	// Apply multiple features
	features := []string{"performance", "security_audit", "compliance"}
	enhancedLogger := registry.Apply(logger, features)
	enhancedLogger.Info("Logger with all features applied")

	// Demonstrate feature behavior
	ctx := context.WithValue(context.Background(), "features", features)
	enhancedLogger.Ctx(ctx).Info("Context-aware logging with features")
}

func demoErrorCategories(logger inbound.Logger, registry inbound.ErrCategories) {
	// Validation error
	valErr := fmt.Errorf("email format is invalid")
	registry.Handle(logger, valErr, "validation", map[string]interface{}{
		"field": "email",
		"value": "not-an-email",
	})

	// Database errors (trigger alert after threshold)
	for i := 0; i < 6; i++ {
		dbErr := fmt.Errorf("connection timeout")
		registry.Handle(logger, dbErr, "database", map[string]interface{}{
			"host":    "db.example.com",
			"port":    5432,
			"attempt": i + 1,
		})

		handler, _ := registry.Get("database")
		if handler.ShouldAlert() {
			log.Printf("ALERT: Database errors exceeded threshold!")
		}
	}

	// Payment error (always alerts)
	payErr := fmt.Errorf("payment gateway timeout")
	registry.Handle(logger, payErr, "payment", map[string]interface{}{
		"transaction_id": "TXN-123",
		"amount":         5000.00,
		"currency":       "THB",
		"gateway":        "stripe",
	})

	handler, _ := registry.Get("payment")
	if handler.ShouldAlert() {
		log.Printf("ALERT: Payment error detected!")
	}
}

// === REGISTRY IMPLEMENTATIONS ===

type FeatureRegistry struct {
	features map[string]inbound.Feature
	mu       sync.RWMutex
}

func NewFeatureRegistry() inbound.Features {
	return &FeatureRegistry{
		features: make(map[string]inbound.Feature),
	}
}

func (r *FeatureRegistry) Register(name string, feature inbound.Feature) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features[name] = feature
	return nil
}

func (r *FeatureRegistry) Get(name string) (inbound.Feature, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	feature, ok := r.features[name]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", name)
	}
	return feature, nil
}

func (r *FeatureRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.features))
	for name := range r.features {
		names = append(names, name)
	}
	return names
}

func (r *FeatureRegistry) Apply(logger inbound.Logger, features []string) inbound.Logger {
	result := logger
	for _, name := range features {
		if feature, err := r.Get(name); err == nil {
			result = feature.Apply(result)
		}
	}
	return result
}

type ErrorCategoryRegistry struct {
	handlers map[string]inbound.ErrHandler
	mu       sync.RWMutex
}

func NewErrorCategoryRegistry() inbound.ErrCategories {
	return &ErrorCategoryRegistry{
		handlers: make(map[string]inbound.ErrHandler),
	}
}

func (r *ErrorCategoryRegistry) Register(category string, handler inbound.ErrHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[category] = handler
	return nil
}

func (r *ErrorCategoryRegistry) Get(category string) (inbound.ErrHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[category]
	if !ok {
		return nil, fmt.Errorf("handler not found: %s", category)
	}
	return handler, nil
}

func (r *ErrorCategoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	categories := make([]string, 0, len(r.handlers))
	for cat := range r.handlers {
		categories = append(categories, cat)
	}
	return categories
}

func (r *ErrorCategoryRegistry) Handle(logger inbound.Logger, err error, category string, details map[string]interface{}) {
	handler, getErr := r.Get(category)
	if getErr != nil {
		// Fallback to generic error logging
		logger.Err(err).Error("Unhandled error category: " + category)
		return
	}
	handler.Handle(logger, err, details)
}

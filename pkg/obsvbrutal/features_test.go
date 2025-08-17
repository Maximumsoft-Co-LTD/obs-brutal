package obsvbrutal_test

import (
	"fmt"
	"sync"
	"testing"

	"obs-brutal/pkg/obsvbrutal"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/inbound"
)

// TestFeaturesInterface ทดสอบ Features interface
func TestFeaturesInterface(t *testing.T) {
	logger, _ := obsvbrutal.NewLogger(obsvbrutal.WithLevel(obsvbrutal.InfoLevel))

	t.Run("Feature Registration", func(t *testing.T) {
		registry := NewTestFeatureRegistry()

		// สร้าง test feature
		testFeature := &TestFeature{name: "test_feature"}

		// Register feature
		err := registry.Register("test", testFeature)
		if err != nil {
			t.Errorf("Failed to register feature: %v", err)
		}

		// Get feature
		feature, err := registry.Get("test")
		if err != nil {
			t.Errorf("Failed to get feature: %v", err)
		}
		if feature.Name() != "test_feature" {
			t.Errorf("Expected feature name 'test_feature', got '%s'", feature.Name())
		}

		// List features
		features := registry.List()
		if len(features) != 1 || features[0] != "test" {
			t.Errorf("Expected features list ['test'], got %v", features)
		}
	})

	t.Run("Feature Application", func(t *testing.T) {
		registry := NewTestFeatureRegistry()

		// Register multiple features
		registry.Register("audit", &AuditFeature{enabled: true})
		registry.Register("metrics", &MetricsFeature{prefix: "app"})
		registry.Register("trace", &TraceFeature{serviceName: "test-service"})

		// Apply single feature
		auditFeature, _ := registry.Get("audit")
		auditLogger := auditFeature.Apply(logger)
		if auditLogger == nil {
			t.Error("Feature.Apply() should return a logger")
		}

		// Apply multiple features
		featureNames := []string{"audit", "metrics", "trace"}
		enhancedLogger := registry.Apply(logger, featureNames)
		if enhancedLogger == nil {
			t.Error("Features.Apply() should return a logger")
		}
	})

	t.Run("Feature Configuration", func(t *testing.T) {
		feature := &ConfigurableFeature{}

		// Configure feature
		config := map[string]interface{}{
			"enabled":   true,
			"threshold": 100.0,
			"tags":      []string{"tag1", "tag2"},
		}

		err := feature.Configure(config)
		if err != nil {
			t.Errorf("Failed to configure feature: %v", err)
		}

		// Verify configuration
		if !feature.enabled {
			t.Error("Feature should be enabled")
		}
		if feature.threshold != 100.0 {
			t.Errorf("Expected threshold 100.0, got %f", feature.threshold)
		}
		if len(feature.tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(feature.tags))
		}
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		registry := NewTestFeatureRegistry()

		// Concurrent registration
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				feature := &TestFeature{name: fmt.Sprintf("feature_%d", id)}
				registry.Register(fmt.Sprintf("feature_%d", id), feature)
			}(i)
		}
		wg.Wait()

		// Verify all registered
		features := registry.List()
		if len(features) != 10 {
			t.Errorf("Expected 10 features, got %d", len(features))
		}

		// Concurrent reading
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_, err := registry.Get(fmt.Sprintf("feature_%d", id))
				if err != nil {
					t.Errorf("Failed to get feature_%d: %v", id, err)
				}
			}(i)
		}
		wg.Wait()
	})
}

// TestErrorCategoriesInterface ทดสอบ ErrCategories interface
func TestErrorCategoriesInterface(t *testing.T) {
	logger, _ := obsvbrutal.NewLogger(obsvbrutal.WithLevel(obsvbrutal.InfoLevel))

	t.Run("Handler Registration", func(t *testing.T) {
		registry := NewTestErrorCategoryRegistry()

		// สร้าง test handler
		handler := &TestErrorHandler{category: "test_error"}

		// Register handler
		err := registry.Register("test", handler)
		if err != nil {
			t.Errorf("Failed to register handler: %v", err)
		}

		// Get handler
		h, err := registry.Get("test")
		if err != nil {
			t.Errorf("Failed to get handler: %v", err)
		}
		if h.Category() != "test_error" {
			t.Errorf("Expected category 'test_error', got '%s'", h.Category())
		}

		// List categories
		categories := registry.List()
		if len(categories) != 1 || categories[0] != "test" {
			t.Errorf("Expected categories ['test'], got %v", categories)
		}
	})

	t.Run("Error Handling", func(t *testing.T) {
		registry := NewTestErrorCategoryRegistry()

		// Register handlers
		registry.Register("validation", &ValidationHandler{})
		registry.Register("database", &DatabaseHandler{threshold: 5})
		registry.Register("payment", &PaymentHandler{})

		// Test validation error
		valErr := fmt.Errorf("invalid email format")
		registry.Handle(logger, valErr, "validation", map[string]interface{}{
			"field": "email",
			"value": "not-an-email",
		})

		// Test database error
		dbErr := fmt.Errorf("connection timeout")
		registry.Handle(logger, dbErr, "database", map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		})

		// Test payment error
		payErr := fmt.Errorf("payment gateway error")
		registry.Handle(logger, payErr, "payment", map[string]interface{}{
			"amount":   1000.00,
			"currency": "THB",
		})
	})

	t.Run("Alert Decision", func(t *testing.T) {
		registry := NewTestErrorCategoryRegistry()

		// Database handler with threshold
		dbHandler := &DatabaseHandler{threshold: 3}
		registry.Register("database", dbHandler)

		// Payment handler (always alerts)
		payHandler := &PaymentHandler{}
		registry.Register("payment", payHandler)

		// Test database errors (should alert after threshold)
		for i := 0; i < 5; i++ {
			handler, _ := registry.Get("database")
			shouldAlert := handler.ShouldAlert()

			if i < 3 && shouldAlert {
				t.Errorf("Should not alert at count %d", i)
			}
			if i >= 3 && !shouldAlert {
				t.Errorf("Should alert at count %d", i)
			}

			// Simulate error
			dbHandler.errorCount++
		}

		// Test payment error (should always alert)
		handler, _ := registry.Get("payment")
		if !handler.ShouldAlert() {
			t.Error("Payment handler should always alert")
		}
	})

	t.Run("Severity Levels", func(t *testing.T) {
		registry := NewTestErrorCategoryRegistry()

		// Register handlers with different severities
		registry.Register("info", &InfoHandler{})
		registry.Register("warning", &WarningHandler{})
		registry.Register("error", &ErrorHandler{})

		// Check severities
		handlers := map[string]domain.Level{
			"info":    domain.InfoLevel,
			"warning": domain.WarnLevel,
			"error":   domain.ErrorLevel,
		}

		for category, expectedLevel := range handlers {
			handler, _ := registry.Get(category)
			if handler.Severity() != expectedLevel {
				t.Errorf("Expected severity %v for %s, got %v",
					expectedLevel, category, handler.Severity())
			}
		}
	})
}

// Test implementations

// Feature implementations
type TestFeature struct {
	name string
}

func (f *TestFeature) Name() string { return f.name }
func (f *TestFeature) Apply(logger inbound.Logger) inbound.Logger {
	return logger.F("feature", f.name)
}
func (f *TestFeature) Configure(config map[string]interface{}) error { return nil }

type AuditFeature struct {
	enabled bool
}

func (f *AuditFeature) Name() string { return "audit" }
func (f *AuditFeature) Apply(logger inbound.Logger) inbound.Logger {
	if f.enabled {
		return logger.F("audit", true).F("audit_time", "2024-01-01")
	}
	return logger
}
func (f *AuditFeature) Configure(config map[string]interface{}) error { return nil }

type MetricsFeature struct {
	prefix string
}

func (f *MetricsFeature) Name() string { return "metrics" }
func (f *MetricsFeature) Apply(logger inbound.Logger) inbound.Logger {
	return logger.F("metrics_prefix", f.prefix)
}
func (f *MetricsFeature) Configure(config map[string]interface{}) error { return nil }

type TraceFeature struct {
	serviceName string
}

func (f *TraceFeature) Name() string { return "trace" }
func (f *TraceFeature) Apply(logger inbound.Logger) inbound.Logger {
	return logger.F("service", f.serviceName).F("trace_enabled", true)
}
func (f *TraceFeature) Configure(config map[string]interface{}) error { return nil }

type ConfigurableFeature struct {
	enabled   bool
	threshold float64
	tags      []string
}

func (f *ConfigurableFeature) Name() string { return "configurable" }
func (f *ConfigurableFeature) Apply(logger inbound.Logger) inbound.Logger {
	return logger.F("configured", true)
}
func (f *ConfigurableFeature) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		f.enabled = enabled
	}
	if threshold, ok := config["threshold"].(float64); ok {
		f.threshold = threshold
	}
	if tags, ok := config["tags"].([]string); ok {
		f.tags = tags
	}
	return nil
}

// Error handler implementations
type TestErrorHandler struct {
	category string
}

func (h *TestErrorHandler) Category() string { return h.category }
func (h *TestErrorHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.Err(err).Error("Test error")
}
func (h *TestErrorHandler) ShouldAlert() bool      { return false }
func (h *TestErrorHandler) Severity() domain.Level { return domain.ErrorLevel }

type ValidationHandler struct{}

func (h *ValidationHandler) Category() string { return "validation" }
func (h *ValidationHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.F("type", "validation").Err(err).Warn("Validation error")
}
func (h *ValidationHandler) ShouldAlert() bool      { return false }
func (h *ValidationHandler) Severity() domain.Level { return domain.WarnLevel }

type DatabaseHandler struct {
	threshold  int
	errorCount int
	mu         sync.Mutex
}

func (h *DatabaseHandler) Category() string { return "database" }
func (h *DatabaseHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	h.mu.Lock()
	h.errorCount++
	h.mu.Unlock()
	logger.F("type", "database").Err(err).Error("Database error")
}
func (h *DatabaseHandler) ShouldAlert() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.errorCount >= h.threshold
}
func (h *DatabaseHandler) Severity() domain.Level { return domain.ErrorLevel }

type PaymentHandler struct{}

func (h *PaymentHandler) Category() string { return "payment" }
func (h *PaymentHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.F("type", "payment").F("critical", true).Err(err).Error("Payment error")
}
func (h *PaymentHandler) ShouldAlert() bool      { return true }
func (h *PaymentHandler) Severity() domain.Level { return domain.ErrorLevel }

type InfoHandler struct{}

func (h *InfoHandler) Category() string { return "info" }
func (h *InfoHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.Info("Info level error")
}
func (h *InfoHandler) ShouldAlert() bool      { return false }
func (h *InfoHandler) Severity() domain.Level { return domain.InfoLevel }

type WarningHandler struct{}

func (h *WarningHandler) Category() string { return "warning" }
func (h *WarningHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.Warn("Warning level error")
}
func (h *WarningHandler) ShouldAlert() bool      { return false }
func (h *WarningHandler) Severity() domain.Level { return domain.WarnLevel }

type ErrorHandler struct{}

func (h *ErrorHandler) Category() string { return "error" }
func (h *ErrorHandler) Handle(logger inbound.Logger, err error, details map[string]interface{}) {
	logger.Error("Error level error")
}
func (h *ErrorHandler) ShouldAlert() bool      { return true }
func (h *ErrorHandler) Severity() domain.Level { return domain.ErrorLevel }

// Registry implementations
type TestFeatureRegistry struct {
	features map[string]inbound.Feature
	mu       sync.RWMutex
}

func NewTestFeatureRegistry() inbound.Features {
	return &TestFeatureRegistry{
		features: make(map[string]inbound.Feature),
	}
}

func (r *TestFeatureRegistry) Register(name string, feature inbound.Feature) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features[name] = feature
	return nil
}

func (r *TestFeatureRegistry) Get(name string) (inbound.Feature, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	feature, ok := r.features[name]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", name)
	}
	return feature, nil
}

func (r *TestFeatureRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.features))
	for name := range r.features {
		names = append(names, name)
	}
	return names
}

func (r *TestFeatureRegistry) Apply(logger inbound.Logger, features []string) inbound.Logger {
	result := logger
	for _, name := range features {
		if feature, err := r.Get(name); err == nil {
			result = feature.Apply(result)
		}
	}
	return result
}

type TestErrorCategoryRegistry struct {
	handlers map[string]inbound.ErrHandler
	mu       sync.RWMutex
}

func NewTestErrorCategoryRegistry() inbound.ErrCategories {
	return &TestErrorCategoryRegistry{
		handlers: make(map[string]inbound.ErrHandler),
	}
}

func (r *TestErrorCategoryRegistry) Register(category string, handler inbound.ErrHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[category] = handler
	return nil
}

func (r *TestErrorCategoryRegistry) Get(category string) (inbound.ErrHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[category]
	if !ok {
		return nil, fmt.Errorf("handler not found: %s", category)
	}
	return handler, nil
}

func (r *TestErrorCategoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	categories := make([]string, 0, len(r.handlers))
	for cat := range r.handlers {
		categories = append(categories, cat)
	}
	return categories
}

func (r *TestErrorCategoryRegistry) Handle(logger inbound.Logger, err error, category string, details map[string]interface{}) {
	handler, getErr := r.Get(category)
	if getErr != nil {
		logger.Err(err).Error("Unhandled error category: " + category)
		return
	}
	handler.Handle(logger, err, details)
}

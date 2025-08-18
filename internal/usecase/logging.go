package usecases

import (
	"context"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	portsIn "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
	portsOut "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/outbound"
)

// LoggingUseCase handles logging business logic
type LoggingUseCase struct {
	logger                portsIn.Logger
	featureRegistry       portsIn.Features
	errorCategoryRegistry portsIn.ErrCategories
	configProvider        portsOut.ConfigSrc
	metricsProvider       MetricsProvider
}

// MetricsProvider for logging metrics
type MetricsProvider interface {
	Log(ctx context.Context, level domain.Level, module string, latency time.Duration) error
	Err(ctx context.Context, module string, errorType string) error
	Get() map[string]interface{}
}

// NewLoggingUseCase creates a new logging use case
func NewLoggingUseCase(
	logger portsIn.Logger,
	featureRegistry portsIn.Features,
	errorCategoryRegistry portsIn.ErrCategories,
	configProvider portsOut.ConfigSrc,
	metricsProvider MetricsProvider,
) *LoggingUseCase {
	return &LoggingUseCase{
		logger:                logger,
		featureRegistry:       featureRegistry,
		errorCategoryRegistry: errorCategoryRegistry,
		configProvider:        configProvider,
		metricsProvider:       metricsProvider,
	}
}

// LogWithContext logs with context and features
func (uc *LoggingUseCase) LogWithContext(ctx context.Context, level domain.Level, msg string, fields map[string]interface{}) error {
	// Check if logging is enabled for this level
	if !uc.shouldLog(ctx, level) {
		return nil
	}

	// Apply context
	logger := uc.logger.Ctx(ctx)

	// Apply features
	features := uc.extractFeatures(ctx)
	for _, featureName := range features {
		if feature, err := uc.featureRegistry.Get(featureName); err == nil {
			logger = feature.Apply(logger)
		}
	}

	// Apply fields
	if len(fields) > 0 {
		logger = logger.Fs(fields)
	}

	// Get module from context
	module := uc.extractModule(ctx)
	if module != "" {
		logger = logger.Mod(module)
	}

	// Log based on level
	switch level {
	case domain.DebugLevel:
		logger.Debug(msg)
	case domain.InfoLevel:
		logger.Info(msg)
	case domain.WarnLevel:
		logger.Warn(msg)
	case domain.ErrorLevel:
		logger.Error(msg)
	case domain.FatalLevel:
		logger.Fatal(msg)
	}

	// Record metrics
	if uc.metricsProvider != nil {
		uc.metricsProvider.Log(ctx, level, module, 0)
	}

	return nil
}

// LogError logs an error with category handling
func (uc *LoggingUseCase) LogErr(ctx context.Context, err error, category string, details map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// Get error handler
	handler, getErr := uc.errorCategoryRegistry.Get(category)
	if getErr != nil {
		// Use default handler
		handler, _ = uc.errorCategoryRegistry.Get("unknown")
	}

	// Apply context
	logger := uc.logger.Ctx(ctx)

	// Handle error
	handler.Handle(logger, err, details)

	// Record metrics
	if uc.metricsProvider != nil {
		uc.metricsProvider.Err(ctx, uc.extractModule(ctx), category)
	}

	// Check if alert is needed
	if handler.ShouldAlert() {
		// Trigger alert (implementation depends on alert system)
		uc.triggerAlert(ctx, err, category, details)
	}

	return nil
}

// LogStruct logs a structured error
func (uc *LoggingUseCase) LogStruct(ctx context.Context, err domain.StructuredError) error {
	// StructuredError is not a pointer type, so we can't check for nil
	// Just proceed with logging

	details := err.GetDetails()
	if details == nil {
		details = make(map[string]interface{})
	}

	// Add structured error fields
	details["error_code"] = err.GetCode()
	details["error_message"] = err.GetMessage()

	return uc.LogErr(ctx, &err, err.GetCategory(), details)
}

// New creates a configured logger
func (uc *LoggingUseCase) New(config portsIn.Config) (portsIn.Logger, error) {
	// Apply default configuration
	if config.Level == 0 {
		config.Level = domain.InfoLevel
	}

	// For now, just return the existing logger
	// In a real implementation, this would create a new logger with the given config
	logger := uc.logger

	// Apply features
	if len(config.Feat) > 0 {
		logger = uc.applyFeatures(logger, config.Feat)
	}

	return logger, nil
}

// Helper methods

func (uc *LoggingUseCase) shouldLog(ctx context.Context, level domain.Level) bool {
	// Get module and tenant from context
	module := uc.extractModule(ctx)
	tenant := uc.extractTenant(ctx)

	// Get configured level
	configuredLevel := uc.configProvider.Level(module, tenant)

	return level >= configuredLevel
}

func (uc *LoggingUseCase) extractModule(ctx context.Context) string {
	if module, ok := ctx.Value("module").(string); ok {
		return module
	}
	return ""
}

func (uc *LoggingUseCase) extractTenant(ctx context.Context) string {
	if tenant, ok := ctx.Value("tenant").(string); ok {
		return tenant
	}
	return ""
}

func (uc *LoggingUseCase) extractFeatures(ctx context.Context) []string {
	if features, ok := ctx.Value("features").([]string); ok {
		return features
	}
	return nil
}

func (uc *LoggingUseCase) applyFeatures(logger portsIn.Logger, features []string) portsIn.Logger {
	for _, featureName := range features {
		if feature, err := uc.featureRegistry.Get(featureName); err == nil {
			logger = feature.Apply(logger)
		}
	}
	return logger
}

func (uc *LoggingUseCase) triggerAlert(ctx context.Context, err error, category string, details map[string]interface{}) {
	uc.logger.Ctx(ctx).Err(err).F("alert_category", category).Fs(details).Error("Alert triggered")
}

// BatchLoggingUseCase handles batch logging operations
type BatchLoggingUseCase struct {
	logger portsIn.Logger
	batch  []batchEntry
	mu     sync.RWMutex
}

type batchEntry struct {
	level   domain.Level
	message string
	fields  map[string]interface{}
}

// NewBatchLoggingUseCase creates a new batch logging use case
func NewBatchLoggingUseCase(logger portsIn.Logger) *BatchLoggingUseCase {
	return &BatchLoggingUseCase{logger: logger, batch: make([]batchEntry, 0)}
}

// Add adds a log entry to the batch
func (b *BatchLoggingUseCase) Add(level domain.Level, message string, fields map[string]interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batch = append(b.batch, batchEntry{level: level, message: message, fields: fields})
}

// Flush flushes all batched logs
func (b *BatchLoggingUseCase) Flush() {
	b.mu.Lock()
	batch := b.batch
	b.batch = make([]batchEntry, 0)
	b.mu.Unlock()
	for _, entry := range batch {
		logger := b.logger
		if len(entry.fields) > 0 {
			logger = logger.Fs(entry.fields)
		}
		switch entry.level {
		case domain.DebugLevel:
			logger.Debug(entry.message)
		case domain.InfoLevel:
			logger.Info(entry.message)
		case domain.WarnLevel:
			logger.Warn(entry.message)
		case domain.ErrorLevel:
			logger.Error(entry.message)
		}
	}
}

// Clear clears the batch without logging
func (b *BatchLoggingUseCase) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batch = make([]batchEntry, 0)
}

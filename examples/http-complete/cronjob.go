package example

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"obs-brutal/pkg/obsvbrutal"

	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/trace"
)

// CronExample demonstrates cron job logging
type CronExample struct {
	logger   obsvbrutal.Logger
	provider *obsvbrutal.OTelProvider
	cron     *cron.Cron
}

// NewCronExample creates cron example
func NewCronExample(
	logger obsvbrutal.Logger,
	provider *obsvbrutal.OTelProvider,
) *CronExample {
	return &CronExample{
		logger:   logger.Mod("cronjob"),
		provider: provider,
		cron:     cron.New(cron.WithSeconds()),
	}
}

// Start starts all cron jobs
func (e *CronExample) Start() error {
	// Add jobs with different schedules
	jobs := []struct {
		schedule string
		name     string
		handler  func()
	}{
		{
			schedule: "*/30 * * * * *", // Every 30 seconds
			name:     "health_check",
			handler:  e.healthCheckJob,
		},
		{
			schedule: "0 * * * * *", // Every minute
			name:     "metrics_aggregation",
			handler:  e.metricsAggregationJob,
		},
		{
			schedule: "0 */5 * * * *", // Every 5 minutes
			name:     "cleanup",
			handler:  e.cleanupJob,
		},
		{
			schedule: "0 0 * * * *", // Every hour
			name:     "report_generation",
			handler:  e.reportGenerationJob,
		},
	}

	for _, job := range jobs {
		jobName := job.name
		handler := job.handler

		// Wrap handler with logging and tracing
		wrappedHandler := func() {
			e.runJob(jobName, handler)
		}

		_, err := e.cron.AddFunc(job.schedule, wrappedHandler)
		if err != nil {
			e.logger.
				Err(err).
				F("job", jobName).
				F("schedule", job.schedule).
				Error("Failed to add cron job")
			return err
		}

		e.logger.
			F("job", jobName).
			F("schedule", job.schedule).
			Info("Cron job registered")
	}

	// Start cron scheduler
	e.cron.Start()
	e.logger.Info("Cron scheduler started")

	return nil
}

// Stop stops all cron jobs
func (e *CronExample) Stop() {
	ctx := e.cron.Stop()
	<-ctx.Done()
	e.logger.Info("Cron scheduler stopped")
}

// runJob runs a job with logging and tracing
func (e *CronExample) runJob(name string, handler func()) {
	start := time.Now()
	jobID := fmt.Sprintf("%s-%d", name, start.Unix())

	// Create logger for this job execution
	jobLogger := e.logger.
		F("job", name).
		F("job_id", jobID).
		F("started_at", start)

	// Start span if OTEL is available
	ctx := context.Background()
	if e.provider != nil {
		var span trace.Span
		ctx, span = obsvbrutal.StartSpan(ctx, fmt.Sprintf("cronjob.%s", name))
		defer span.End()

		// Update logger with trace context
		jobLogger = jobLogger.Ctx(ctx)
	}

	jobLogger.Info("Cron job started")

	// Run the actual job
	defer func() {
		if r := recover(); r != nil {
			jobLogger.
				F("panic", r).
				Error("Cron job panicked")
		}
	}()

	handler()

	// Log completion
	duration := time.Since(start)
	jobLogger.
		F("duration_ms", duration.Milliseconds()).
		Info("Cron job completed")
}

// Job implementations

func (e *CronExample) healthCheckJob() {
	// Simulate health checks
	services := []string{"database", "redis", "rabbitmq", "external_api"}

	for _, service := range services {
		// Simulate checking service
		latency := time.Duration(10+rand.Intn(50)) * time.Millisecond
		time.Sleep(latency)

		// Random health status
		healthy := rand.Float32() > 0.1

		if healthy {
			e.logger.
				F("service", service).
				F("latency_ms", latency.Milliseconds()).
				Debug("Health check passed")
		} else {
			e.logger.
				F("service", service).
				F("latency_ms", latency.Milliseconds()).
				Warn("Health check failed")
		}
	}
}

func (e *CronExample) metricsAggregationJob() {
	// Simulate metrics aggregation
	metrics := map[string]interface{}{
		"orders_count":      rand.Intn(100),
		"revenue":           rand.Float64() * 10000,
		"active_users":      rand.Intn(1000),
		"response_time_p95": rand.Intn(500),
		"error_rate":        rand.Float64() * 0.05,
	}

	e.logger.
		Fs(metrics).
		Info("Metrics aggregated")

	// Simulate storing metrics
	time.Sleep(50 * time.Millisecond)
}

func (e *CronExample) cleanupJob() {
	// Simulate cleanup operations
	e.logger.Debug("Starting cleanup operations")

	// Clean old logs
	deletedLogs := rand.Intn(1000)
	e.logger.
		F("deleted_count", deletedLogs).
		F("type", "logs").
		Debug("Cleaned old logs")

	// Clean expired sessions
	deletedSessions := rand.Intn(50)
	e.logger.
		F("deleted_count", deletedSessions).
		F("type", "sessions").
		Debug("Cleaned expired sessions")

	// Clean temporary files
	deletedFiles := rand.Intn(200)
	e.logger.
		F("deleted_count", deletedFiles).
		F("type", "temp_files").
		Debug("Cleaned temporary files")

	totalDeleted := deletedLogs + deletedSessions + deletedFiles
	e.logger.
		F("total_deleted", totalDeleted).
		Info("Cleanup completed")
}

func (e *CronExample) reportGenerationJob() {
	// Simulate report generation
	reportTypes := []string{"sales", "inventory", "user_activity", "system_performance"}

	for _, reportType := range reportTypes {
		reportID := fmt.Sprintf("report-%s-%d", reportType, time.Now().Unix())

		reportLogger := e.logger.
			F("report_id", reportID).
			F("report_type", reportType)

		reportLogger.Info("Generating report")

		// Simulate report generation
		time.Sleep(time.Duration(100+rand.Intn(400)) * time.Millisecond)

		// Simulate occasional failure
		if rand.Float32() < 0.05 {
			reportLogger.Error("Report generation failed")
			continue
		}

		reportLogger.
			F("size_kb", rand.Intn(1000)).
			F("row_count", rand.Intn(10000)).
			Info("Report generated successfully")
	}
}

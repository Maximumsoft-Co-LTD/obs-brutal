package obsvbrutal

import (
	"context"
	"fmt"
	"net/http"
	"obs-brutal/pkg/config"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Module provides FX module for Log Brutal
var Module = fx.Module("obsvbrutal",
	fx.Provide(
		NewFxLogger,
		NewFxOTelProvider,
		NewFxRedisConfigSrc,
		NewFxDynamicLogger,
		// Provide a dedicated metrics HTTP server under a name to avoid conflicts
		fx.Annotate(
			NewFxHTTPServer,
			fx.ResultTags(`name:"metrics_server"`),
		),
	),
	fx.Invoke(
		RegisterLifecycle,
		RegisterMetricsEndpoint,
	),
)

// ModuleParams configures the Log Brutal module
type ModuleParams struct {
	fx.In
	Config *config.ObsConfig
}

// FxLoggerResult provides logger outputs
type FxLoggerResult struct {
	fx.Out
	Logger    Logger
	ZapLogger *zap.Logger
}

// NewFxLogger creates logger for FX
func NewFxLogger(params ModuleParams) (FxLoggerResult, error) {
	// Parse log level
	level := InfoLevel
	if params.Config.Log.LEVEL != "" {
		level = ParseLevel(params.Config.Log.LEVEL)
	}

	// Create sinks
	var sinks []Sink

	// Always add stdout
	sinks = append(sinks, NewStdoutSink())

	// Add file sink if configured
	if params.Config.Log.FILE != "" {
		fileSink := NewFileSink(
			params.Config.Log.FILE,
			100,  // 100MB max size
			30,   // 30 days retention
			10,   // 10 backup files
			true, // compress
		)
		sinks = append(sinks, NewBufferedSink(fileSink, 1000, 100*time.Millisecond))
	}

	// Add OTLP sink if configured
	if params.Config.Otel.ENABLED && params.Config.Otel.ENDPOINT != "" {
		otlpSink, err := NewOTLPSink(params.Config.Otel.ENDPOINT, true)
		if err != nil {
			return FxLoggerResult{}, fmt.Errorf("failed to create OTLP sink: %w", err)
		}
		sinks = append(sinks, otlpSink)
	}

	// Add Loki sink if configured
	if params.Config.Loki.ENABLED && params.Config.Loki.URL != "" {
		lokiLabels := map[string]string{
			"service": params.Config.AppSrv.SERVICE,
			"env":     "production",
		}
		lokiSink := NewLokiSink(params.Config.Loki.URL, lokiLabels, 100)
		sinks = append(sinks, NewBufferedSink(lokiSink, 1000, 500*time.Millisecond))
	}

	// Create multiplex sink
	multiplexSink := NewMultiplexSink(sinks...)

	// Create logger
	brutalLogger, err := NewLogger(
		WithLevel(level),
		WithSinks(multiplexSink),
	)
	if err != nil {
		return FxLoggerResult{}, err
	}

	// Add default fields
	brutalLogger = brutalLogger.
		F("service", params.Config.AppSrv.SERVICE).
		F("environment", params.Config.AppSrv.ENVIRONMENT)

	// Create zap logger separately for FX
	zapConfig := zap.NewProductionConfig()
	zapConfig.Level = zap.NewAtomicLevelAt(zapcore.Level(level))
	zapLogger, err := zapConfig.Build()
	if err != nil {
		return FxLoggerResult{}, fmt.Errorf("failed to create zap logger: %w", err)
	}

	return FxLoggerResult{
		Logger:    brutalLogger,
		ZapLogger: zapLogger,
	}, nil
}

// FxOTelProviderResult provides OTEL outputs
type FxOTelProviderResult struct {
	fx.Out

	Provider *OTelProvider `optional:"true"`
	// OTelLogger deprecated - use context propagation instead
}

// NewFxOTelProvider creates OpenTelemetry provider for FX
func NewFxOTelProvider(params ModuleParams, logger Logger) (FxOTelProviderResult, error) {
	if !params.Config.Otel.ENABLED || params.Config.Otel.ENDPOINT == "" {
		return FxOTelProviderResult{}, nil
	}

	provider, err := NewOTelProvider(params.Config.Otel.SERVICE_NAME, params.Config.Otel.ENDPOINT, true)
	if err != nil {
		return FxOTelProviderResult{}, fmt.Errorf("failed to create OTel provider: %w", err)
	}

	// Return provider only - logger integration via context
	return FxOTelProviderResult{
		Provider: provider,
	}, nil
}

// FxRedisConfigSrcResult provides Redis config outputs
type FxRedisConfigSrcResult struct {
	fx.Out

	ConfigSrc   ConfigSrc     `optional:"true"`
	RedisClient *redis.Client `optional:"true"`
}

// NewFxRedisConfigSrc creates Redis config provider for FX
func NewFxRedisConfigSrc(params ModuleParams) (FxRedisConfigSrcResult, error) {
	if params.Config.Database.Redis.ADDR == "" {
		return FxRedisConfigSrcResult{}, nil
	}
	fmt.Println("params.RedisAddr", params.Config.Database.Redis.ADDR)
	client := redis.NewClient(&redis.Options{
		Addr:         params.Config.Database.Redis.ADDR,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		// Redis is optional, log warning and return nil
		fmt.Printf("WARNING: Failed to connect to Redis at %s: %v. Running without dynamic config.\n", params.Config.Database.Redis.ADDR, err)
		return FxRedisConfigSrcResult{}, nil
	}

	configSrc, err := NewRedisConfigProvider(params.Config.Database.Redis.ADDR, params.Config.Database.Redis.PASSWORD, params.Config.Database.Redis.DB, "obsvbrutal:")
	if err != nil {
		return FxRedisConfigSrcResult{}, fmt.Errorf("failed to create Redis config provider: %w", err)
	}

	return FxRedisConfigSrcResult{
		ConfigSrc:   configSrc,
		RedisClient: client,
	}, nil
}

// FxDynamicLoggerResult provides dynamic logger output
type FxDynamicLoggerResult struct {
	fx.Out

	DynamicLogger Logger `name:"dynamic_logger" optional:"true"`
}

// NewFxDynamicLogger creates dynamic logger for FX
func NewFxDynamicLogger(
	baseLogger Logger,
	configSrc ConfigSrc,
) (FxDynamicLoggerResult, error) {
	if configSrc == nil {
		return FxDynamicLoggerResult{
			DynamicLogger: baseLogger,
		}, nil
	}

	// For now, just return the base logger
	// TODO: Implement dynamic logger with config provider integration
	return FxDynamicLoggerResult{
		DynamicLogger: baseLogger,
	}, nil
}

// HTTPServerParams for metrics server
type HTTPServerParams struct {
	fx.In

	MetricsEnabled bool          `name:"metrics_enabled" optional:"true"`
	MetricsPort    int           `name:"metrics_port" optional:"true"`
	Provider       *OTelProvider `optional:"true"`
}

// NewFxHTTPServer creates HTTP server for metrics
func NewFxHTTPServer(params HTTPServerParams) *http.Server {
	if !params.MetricsEnabled {
		return nil
	}

	port := params.MetricsPort
	if port == 0 {
		port = 9090
	}

	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Ready check
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// RegisterLifecycle registers lifecycle hooks
type lifecycleParams struct {
	fx.In
	Config      *config.ObsConfig
	Lifecycle   fx.Lifecycle
	Logger      Logger
	MetricsHTTP *http.Server `name:"metrics_server"`
	Provider    *OTelProvider
	RedisClient *redis.Client
}

func RegisterLifecycle(params lifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			params.Logger.Info("Starting Log Brutal system")

			// Start metrics server
			if params.MetricsHTTP != nil {
				go startMetricsServerWithFallback(params.MetricsHTTP, params.Logger)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Stopping Log Brutal system")

			// Shutdown metrics server
			if params.MetricsHTTP != nil {
				if err := params.MetricsHTTP.Shutdown(ctx); err != nil {
					params.Logger.Err(err).Error("Failed to shutdown metrics server")
				}
			}

			// Shutdown OTel provider
			if params.Provider != nil {
				if err := params.Provider.Shutdown(ctx); err != nil {
					params.Logger.Err(err).Error("Failed to shutdown OTel provider")
				}
			}

			// Close Redis
			if params.RedisClient != nil {
				if err := params.RedisClient.Close(); err != nil {
					params.Logger.Err(err).Error("Failed to close Redis client")
				}
			}

			// Sync logger - no longer needed with interface-based approach

			return nil
		},
	})
}

// RegisterMetricsEndpoint for debugging
type metricsEndpointParams struct {
	fx.In

	Server *http.Server `name:"metrics_server"`
	Logger Logger
}

func RegisterMetricsEndpoint(params metricsEndpointParams) {
	if params.Server != nil {
		params.Logger.F("port", params.Server.Addr).Info("Metrics endpoint available at /metrics")
	}
}

// Gin integration

// GinModule provides Gin integration
var GinModule = fx.Module("obsvbrutal-gin",
	fx.Provide(
		NewGinMiddleware,
		NewGinRouter,
	),
)

// GinMiddlewareParams for Gin middleware
type GinMiddlewareParams struct {
	fx.In

	Logger   Logger        `name:"dynamic_logger"`
	Provider *OTelProvider `optional:"true"`
}

// GinMiddlewareResult provides Gin middleware
type GinMiddlewareResult struct {
	fx.Out

	LogMiddleware  gin.HandlerFunc `name:"log_middleware"`
	OTelMiddleware gin.HandlerFunc `name:"otel_middleware" optional:"true"`
}

// NewGinMiddleware creates Gin middleware
func NewGinMiddleware(params GinMiddlewareParams) GinMiddlewareResult {
	result := GinMiddlewareResult{
		LogMiddleware: GinMiddleware(params.Logger),
	}

	if params.Provider != nil {
		result.OTelMiddleware = OTelGinMiddleware(params.Logger, params.Provider)
	}

	return result
}

// NewGinRouter creates Gin router with middleware
func NewGinRouter(
	logMiddleware gin.HandlerFunc,
	otelMiddleware gin.HandlerFunc,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// Add OTel middleware first if available
	if otelMiddleware != nil {
		router.Use(otelMiddleware)
	} else {
		router.Use(logMiddleware)
	}

	return router
}

// WithLogger provides logger to FX app
func WithLogger(logger Logger) fx.Option {
	return fx.Provide(func() Logger {
		return logger
	})
}

// AsLogger tags logger for injection
func AsLogger() fx.Option {
	return fx.Provide(
		fx.Annotate(
			func(logger Logger) Logger { return logger },
			fx.As(new(Logger)),
		),
	)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return defaultValue
}

// startMetricsServerWithFallback starts metrics server and if the port is busy, it increments the port until available
func startMetricsServerWithFallback(server *http.Server, logger Logger) {
	addr := server.Addr
	const maxAttempts = 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		logger.F("addr", addr).Info("Starting metrics server")
		server.Addr = addr
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if strings.Contains(err.Error(), "address already in use") {
				portStr := strings.TrimPrefix(addr, ":")
				p, convErr := strconv.Atoi(portStr)
				if convErr != nil {
					logger.Err(convErr).Fatal("Invalid metrics port")
					return
				}
				p++
				addr = ":" + strconv.Itoa(p)
				continue
			}
			logger.Err(err).Error("Metrics server error")
			return
		}
		return
	}
	logger.F("addr", addr).Error("Failed to bind metrics server after attempts")
}

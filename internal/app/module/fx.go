package module

import (
	"context"
	"fmt"
	"net/http"
	cfg "obs-brutal/internal/app/config"
	obsv "obs-brutal/obsvbrutal"
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
		fx.Annotate(NewFxHTTPServer, fx.ResultTags(`name:"metrics_server"`)),
	),
	fx.Invoke(RegisterLifecycle, RegisterMetricsEndpoint),
)

type ModuleParams struct {
	fx.In
	Config *cfg.ObsConfig
}

type FxLoggerResult struct {
	fx.Out
	Logger    obsv.Logger
	ZapLogger *zap.Logger
}

func NewFxLogger(params ModuleParams) (FxLoggerResult, error) {
	level := obsv.InfoLevel
	if params.Config.Log.LEVEL != "" {
		level = obsv.ParseLevel(params.Config.Log.LEVEL)
	}
	var sinks []obsv.Sink
	sinks = append(sinks, obsv.NewStdoutSink())
	if params.Config.Log.FILE != "" {
		fileSink := obsv.NewFileSink(params.Config.Log.FILE, 100, 30, 10, true)
		sinks = append(sinks, obsv.NewBufferedSink(fileSink, 1000, 100*time.Millisecond))
	}
	if params.Config.Otel.ENABLED && params.Config.Otel.ENDPOINT != "" {
		otlpSink, err := obsv.NewOTLPSink(params.Config.Otel.ENDPOINT, true)
		if err != nil {
			return FxLoggerResult{}, fmt.Errorf("failed to create OTLP sink: %w", err)
		}
		sinks = append(sinks, otlpSink)
	}
	if params.Config.Loki.ENABLED && params.Config.Loki.URL != "" {
		lokiLabels := map[string]string{"service": params.Config.AppSrv.SERVICE, "env": "production"}
		lokiSink := obsv.NewLokiSink(params.Config.Loki.URL, lokiLabels, 100)
		sinks = append(sinks, obsv.NewBufferedSink(lokiSink, 1000, 500*time.Millisecond))
	}
	multiplexSink := obsv.NewMultiplexSink(sinks...)
	brutalLogger, err := obsv.NewLogger(obsv.WithLevel(level), obsv.WithSinks(multiplexSink))
	if err != nil {
		return FxLoggerResult{}, err
	}
	brutalLogger = brutalLogger.F("service", params.Config.AppSrv.SERVICE).F("environment", params.Config.AppSrv.ENVIRONMENT)
	zapConfig := zap.NewProductionConfig()
	zapConfig.Level = zap.NewAtomicLevelAt(zapcore.Level(level))
	zapLogger, err := zapConfig.Build()
	if err != nil {
		return FxLoggerResult{}, fmt.Errorf("failed to create zap logger: %w", err)
	}
	return FxLoggerResult{Logger: brutalLogger, ZapLogger: zapLogger}, nil
}

type FxOTelProviderResult struct {
	fx.Out
	Provider *obsv.OTelProvider `optional:"true"`
}

func NewFxOTelProvider(params ModuleParams, logger obsv.Logger) (FxOTelProviderResult, error) {
	if !params.Config.Otel.ENABLED || params.Config.Otel.ENDPOINT == "" {
		return FxOTelProviderResult{}, nil
	}
	provider, err := obsv.NewOTelProvider(params.Config.Otel.SERVICE_NAME, params.Config.Otel.ENDPOINT, true)
	if err != nil {
		return FxOTelProviderResult{}, fmt.Errorf("failed to create OTel provider: %w", err)
	}
	return FxOTelProviderResult{Provider: provider}, nil
}

type FxRedisConfigSrcResult struct {
	fx.Out
	ConfigSrc   obsv.ConfigSrc `optional:"true"`
	RedisClient *redis.Client  `optional:"true"`
}

func NewFxRedisConfigSrc(params ModuleParams) (FxRedisConfigSrcResult, error) {
	if params.Config.Database.Redis.ADDR == "" {
		return FxRedisConfigSrcResult{}, nil
	}
	client := redis.NewClient(&redis.Options{Addr: params.Config.Database.Redis.ADDR, DialTimeout: 5 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, PoolSize: 10, MinIdleConns: 5})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Printf("WARNING: Failed to connect to Redis at %s: %v. Running without dynamic config.\n", params.Config.Database.Redis.ADDR, err)
		return FxRedisConfigSrcResult{}, nil
	}
	configSrc, err := obsv.NewRedisConfigProvider(params.Config.Database.Redis.ADDR, params.Config.Database.Redis.PASSWORD, params.Config.Database.Redis.DB, "obsvbrutal:")
	if err != nil {
		return FxRedisConfigSrcResult{}, fmt.Errorf("failed to create Redis config provider: %w", err)
	}
	return FxRedisConfigSrcResult{ConfigSrc: configSrc, RedisClient: client}, nil
}

type FxDynamicLoggerResult struct {
	fx.Out
	DynamicLogger obsv.Logger `name:"dynamic_logger" optional:"true"`
}

func NewFxDynamicLogger(baseLogger obsv.Logger, configSrc obsv.ConfigSrc) (FxDynamicLoggerResult, error) {
	if configSrc == nil {
		return FxDynamicLoggerResult{DynamicLogger: baseLogger}, nil
	}
	return FxDynamicLoggerResult{DynamicLogger: baseLogger}, nil
}

type HTTPServerParams struct {
	fx.In
	MetricsEnabled bool               `name:"metrics_enabled" optional:"true"`
	MetricsPort    int                `name:"metrics_port" optional:"true"`
	Provider       *obsv.OTelProvider `optional:"true"`
}

func NewFxHTTPServer(params HTTPServerParams) *http.Server {
	if !params.MetricsEnabled {
		return nil
	}
	port := params.MetricsPort
	if port == 0 {
		port = 9090
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("OK")) })
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("READY")) })
	return &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
}

type lifecycleParams struct {
	fx.In
	Config      *cfg.ObsConfig
	Lifecycle   fx.Lifecycle
	Logger      obsv.Logger
	MetricsHTTP *http.Server `name:"metrics_server"`
	Provider    *obsv.OTelProvider
	RedisClient *redis.Client
}

func RegisterLifecycle(params lifecycleParams) {
	params.Lifecycle.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		params.Logger.Info("Starting Log Brutal system")
		if params.MetricsHTTP != nil {
			go startMetricsServerWithFallback(params.MetricsHTTP, params.Logger)
		}
		return nil
	}, OnStop: func(ctx context.Context) error {
		params.Logger.Info("Stopping Log Brutal system")
		if params.MetricsHTTP != nil {
			if err := params.MetricsHTTP.Shutdown(ctx); err != nil {
				params.Logger.Err(err).Error("Failed to shutdown metrics server")
			}
		}
		if params.Provider != nil {
			if err := params.Provider.Shutdown(ctx); err != nil {
				params.Logger.Err(err).Error("Failed to shutdown OTel provider")
			}
		}
		if params.RedisClient != nil {
			if err := params.RedisClient.Close(); err != nil {
				params.Logger.Err(err).Error("Failed to close Redis client")
			}
		}
		return nil
	}})
}

type metricsEndpointParams struct {
	fx.In
	Server *http.Server `name:"metrics_server"`
	Logger obsv.Logger
}

func RegisterMetricsEndpoint(params metricsEndpointParams) {
	if params.Server != nil {
		params.Logger.F("port", params.Server.Addr).Info("Metrics endpoint available at /metrics")
	}
}

var GinModule = fx.Module("obsvbrutal-gin", fx.Provide(NewGinMiddleware, NewGinRouter))

type GinMiddlewareParams struct {
	fx.In
	Logger   obsv.Logger        `name:"dynamic_logger"`
	Provider *obsv.OTelProvider `optional:"true"`
}

type GinMiddlewareResult struct {
	fx.Out
	LogMiddleware  gin.HandlerFunc `name:"log_middleware"`
	OTelMiddleware gin.HandlerFunc `name:"otel_middleware" optional:"true"`
}

func NewGinMiddleware(params GinMiddlewareParams) GinMiddlewareResult {
	result := GinMiddlewareResult{LogMiddleware: obsv.GinMiddleware(params.Logger)}
	// OTelGinMiddleware not wired here; use log middleware or wire your own OTel middleware externally.
	return result
}

func NewGinRouter(logMiddleware gin.HandlerFunc, otelMiddleware gin.HandlerFunc, prometheusMiddleware gin.HandlerFunc, metricsMiddleware gin.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	if otelMiddleware != nil {
		router.Use(otelMiddleware)
	} else {
		router.Use(logMiddleware)
	}
	if prometheusMiddleware != nil {
		router.Use(prometheusMiddleware)
	}
	if metricsMiddleware != nil {
		router.Use(metricsMiddleware)
	}
	return router
}

func WithLogger(logger obsv.Logger) fx.Option {
	return fx.Provide(func() obsv.Logger { return logger })
}

func AsLogger() fx.Option {
	return fx.Provide(fx.Annotate(func(logger obsv.Logger) obsv.Logger { return logger }, fx.As(new(obsv.Logger))))
}

func startMetricsServerWithFallback(server *http.Server, logger obsv.Logger) {
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

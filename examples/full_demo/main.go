package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"obs-brutal/internal/core"
	"obs-brutal/logtrc"
)

func main() {
	// ---- Build sinks (stdout buffer, file rotation, OTLP, Loki, Zerolog) ----
	file := logtrc.NewOptimalFileSink()
	_ = file.Configure(map[string]interface{}{
		"filename":          "logs/full_demo.log",
		"rotate_size_bytes": int64(5 << 20), // 5MB
		"max_backups":       3,
	})

	otlp := logtrc.NewOTLPSink("localhost:4317")
	_ = otlp.Configure(map[string]interface{}{
		"endpoint": "localhost:4317",
		"resource": map[string]string{"service.name": "full-demo", "env": "dev"},
	})

	loki := logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{
		"app": "obs-brutal", "env": "dev", "component": "full-demo",
	})

	sinks := []logtrc.Sink{
		logtrc.NewBufferedSink(),
		file,
		otlp,
		loki,
		logtrc.NewZerologSink(),
	}

	// ---- Async logger with multiple sinks ----
	log := logtrc.NewAsyncLogBrt(logtrc.DEBUG, sinks...)

	// ---- Fluent API coverage: F, Fs, Ctx, TraceID, UserID, RequestID, WithError ----
	baseCtx := context.WithValue(context.Background(), "user_id", "u-1001")
	baseCtx = context.WithValue(baseCtx, "request_id", "req-xyz")

	log.
		F("module", "full_demo").
		Fs(map[string]interface{}{"feature": "fluent", "ok": true}).
		Ctx(baseCtx).
		TraceID("deadbeefdeadbeefdeadbeefdeadbeef").
		UserID("u-1001").
		RequestID("req-xyz").
		WithError(errors.New("sample error")).
		Info("fluent fields covered")

	// ---- Formatted methods coverage ----
	log.Debugf("debug num=%d", 1)
	log.Infof("info str=%s", "hello")
	log.Warnf("warn flag=%v", true)
	log.Errorf("error code=%d", 500)
	// log.Fatalf("fatal example") // avoid exiting the program in example

	// ---- Strategy: filter + sampler + masker (direct usage) ----
	strat := core.NewStrategyLogBrt(core.DEBUG, sinks...)
	strat.AddFilter(core.NewLevelFilter(core.INFO, core.ERROR))
	strat.AddSampler(core.NewRateSampler(0.5)) // 50%
	strat.AddMasker(core.NewPIIMasker())
	strat.F("email", "john.doe@example.com").Info("strategy engine applied")

	// ---- Security mode with enterprise masking + audit ----
	if sec, err := core.NewSecurityLogBrt("full-demo", "1.0.0", "dev", "localhost:4317", core.INFO, sinks...); err == nil && sec != nil {
		secWithFields := sec.F("email", "john.doe@example.com").F("phone", "+66-89-123-4567")
		if s, ok := secWithFields.(*core.SecurityLogBrt); ok {
			s.LogWithSecurity(core.INFO, "security log with access control", "u-1001", "admin", true)
		} else {
			sec.LogWithSecurity(core.INFO, "security log with access control", "u-1001", "admin", true)
		}
	} else {
		fmt.Println("security logger unavailable, skipping security demo")
	}

	// ---- AMQP Context propagation demo ----
	prop := core.NewAMQPContextPropagator()
	ctx := context.Background()
	// simulate having an active span in ctx via OTel provider (skipped here). We still show carrier usage.
	headers := map[string]interface{}{}
	prop.InjectAMQPHeaders(ctx, headers)
	// later at consumer:
	ctx2 := prop.ExtractAMQPHeaders(context.Background(), headers)
	log.Ctx(ctx2).F("amqp", true).Info("amqp context propagated")

	// ---- Final simple logs ----
	for i := 0; i < 3; i++ {
		log.F("iteration", i).Info("looping")
		time.Sleep(50 * time.Millisecond)
	}
}

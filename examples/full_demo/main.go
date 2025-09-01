package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "obs-brutal/internal/util"
    "obs-brutal/logtrc"
)

func main() {
	// ---- Build sinks (stdout buffer, file rotation, Loki, Zerolog) ----
	file := logtrc.NewOptimalFileSink()
	_ = file.Configure(map[string]interface{}{
		"filename":          "logs/full_demo.log",
		"rotate_size_bytes": int64(5 << 20), // 5MB
		"max_backups":       3,
	})

	loki := logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{
		"app": "obs-brutal", "env": "dev", "component": "full-demo",
	})

	sinks := []logtrc.Sink{
		logtrc.NewBufferedSink(),
		file,
		loki,
		logtrc.NewZerologSink(),
	}

	// ---- Async logbrut with multiple sinks ----
	log := logtrc.NewAsyncLogBrt(logtrc.DEBUG, sinks...)

	// ---- Fluent API coverage: F, Fs, Ctx, TraceID, UserID, RequestID, WithError ----
    baseCtx := util.WithUserID(context.Background(), "u-1001")
    baseCtx = util.WithRequestID(baseCtx, "req-xyz")
    baseCtx = util.WithTraceID(baseCtx, "trace-full-demo-001")

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

	// ---- Security mode with enterprise masking + audit ----
    if sec, err := logtrc.NewSecurityLogBrt("full-demo", "1.0.0", "dev", "localhost:4317", logtrc.INFO, sinks...); err == nil && sec != nil {
        secWithFields := sec.F("email", "john.doe@example.com").F("phone", "+66-89-123-4567")
        if s, ok := secWithFields.(*logtrc.SecurityLogBrt); ok {
            s.LogWithSecurity(logtrc.INFO, "security log with access control", "u-1001", "admin", true)
        } else {
            sec.LogWithSecurity(logtrc.INFO, "security log with access control", "u-1001", "admin", true)
        }
    } else {
        fmt.Println("security logbrut unavailable, skipping security demo")
    }

	// ---- AMQP Context propagation demo (placeholder) ----
	ctx := context.Background()
	headers := map[string]interface{}{}
	_ = headers
	_ = ctx

	// ---- Final simple logs ----
	for i := 0; i < 3; i++ {
		log.F("iteration", i).Info("looping")
		time.Sleep(50 * time.Millisecond)
	}
}

package main

import (
    "time"

    "obs-brutal/logtrc"
)

func main() {
    sinks := []logtrc.Sink{
        logtrc.NewConsoleSink(true),
        logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{
            "app": "obs-brutal", "env": "dev",
        }),
        logtrc.NewZerologSink(),
    }

    // Use OTEL with service set for metrics labels (and Loki sink)
    var logger logtrc.LogBrt
    if ot, _, err := logtrc.NewOTelWithService("example-otel-loki", "1.0.0", "dev", "localhost:4317", logtrc.INFO, sinks...); err == nil && ot != nil {
        logger = ot
    } else {
        logger = logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)
    }

    // Simple logs
    logger.F("module", "example").Info("hello world")

    // Structured logs
    for i := 0; i < 5; i++ {
        logger.F("iteration", i).
            F("success", true).
            F("tenant", "t1").
            Info("processing done")
        time.Sleep(100 * time.Millisecond)
    }
}

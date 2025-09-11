package main

import (
    "time"

    "obs-brutal/logtrc"
)

// Demo: write logs to local file under ./logs/, so Promtail (in compose)
// tails the files and ships them to Loki. Open Grafana -> Explore -> Loki
// and query: {job="obs-brutal"}
func main() {
    // Configure rolling file sink at ./logs/app.log
    file := logtrc.NewLumberjackSink()
    _ = file.Configure(map[string]interface{}{
        "filename":     "logs/app.log",
        "max_size_mb":  50,
        "max_backups":  7,
        "max_age_days": 14,
        "compress":     true,
    })

    // Buffer the file sink for better throughput
    buf := logtrc.NewBufferedWrap(file, 1000, 100*time.Millisecond)

    // Create async logger with the buffered file sink
    log := logtrc.NewAsyncCfg(2000, 4, 100*time.Millisecond, logtrc.INFO, buf)

    log.F("module", "promtail_demo").F("env", "dev").Info("file logging started")
    for i := 0; i < 5; i++ {
        log.F("i", i).Info("hello from promtail demo")
        time.Sleep(200 * time.Millisecond)
    }
    log.F("module", "promtail_demo").Info("file logging done")

    // Allow async flush before exit (small sleep or keep process alive in real app)
    time.Sleep(500 * time.Millisecond)
}


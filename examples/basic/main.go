package main

import (
    "obs-brutal/logtrc"
)

func main() {
    // Prefer OTEL with service label if available; fallback to default
    if ot, _, err := logtrc.NewOTelWithService("example-basic", "1.0.0", "dev", "localhost:4317", logtrc.INFO); err == nil && ot != nil {
        ot.F("environment", "dev").Info("hello from examples/basic (otel)")
        ot.F("environment", "dev").F("user_id", 123).Info("structured log")
        return
    }
    log := logtrc.NewDefault()
    log.F("environment", "dev").Info("hello from examples/basic")
    log.F("environment", "dev").F("user_id", 123).Info("structured log")
}

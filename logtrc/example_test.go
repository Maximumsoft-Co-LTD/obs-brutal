package logtrc

import (
    "context"
)

// Example for basic facade usage
func Example_basic() {
    log := NewDefault()
    log.F("module", "example").Info("hello")
}

// Example for async + OTEL helper (provider may not be available in tests; just compile)
func Example_asyncWithOTel() {
    if al, err := NewAsyncWithOTel("svc", "1.0.0", "dev", "localhost:4317", INFO); err == nil {
        _ = al
    }
}

// Example to extract OTEL-aware logger from Gin (compile-only snippet)
func Example_OTelLogger() {
    var ctx context.Context
    _ = ctx
}

// Example showing most public methods from logtrc facade
func Example_methods() {
    // Constructors
    _ = NewDefault()
    _ = New(Async(true)) // prefer options

    // Sinks
    _ = NewFastStdoutSink()
    _ = NewBufferedSink()
    _ = NewBufferedSinkWith(1000, 100000000) // 100ms
    _ = NewOptimalFileSink()
    _ = NewZerologSink()

    // OTEL helpers
    if ot, _, err := NewOTelWithService("svc", "1.0.0", "dev", "localhost:4317", INFO); err == nil {
        _ = ot
    }
    if al, err := NewAsyncWithOTel("svc", "1.0.0", "dev", "localhost:4317", INFO); err == nil {
        _ = al
    }

    // Response helpers (compile-only – require Gin context at runtime)
    // rb := GetLogTrcFrmGin(c, "op").R(200, Opts.Msg("ok"))
    // rb.Send()

    // Convenience functions
    Debug("d"); Info("i"); Warn("w"); Error("e")
}

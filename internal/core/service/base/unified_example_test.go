package base

import (
    "context"
)

// Example demonstrating core fluent methods on UnifiedLogBrt.
func ExampleUnifiedLogBrt_fluent() {
    log := NewUnifiedLogBrt(0)
    ctx := context.Background()
    log.
        F("module", "example").
        Fs(map[string]interface{}{"ok": true}).
        Ctx(ctx).
        TraceID("t123").
        UserID("u1").
        RequestID("r1").
        WithError(nil).
        Info("hello")
}


# Observability walkthrough

This document answers one question for every scenario:

> **"If I write this code, what exactly will I see in Trace, Log, and Metrics?"**

Every JSON line below is **real output** — it is captured by
[`boeng/walkthrough_test.go`](../boeng/walkthrough_test.go), which runs
the same scenarios via the public API and asserts that:

1. the boeng runtime produces a log entry carrying these exact
   `(msg, level, op, fields)` tuples, AND
2. this document contains a JSON line carrying the same tuples.

If either side drifts, the test fails. No invented outputs.

Timestamps and durations are real but vary per run; the test ignores
them. Everything else is exact.

---

> All log lines below use the default `Config.Level` (nothing filtered)
> and default levels: `started` DEBUG, `completed` INFO, `failed` ERROR,
> `Emit` INFO. `Config.QuietOps` moves `completed` to DEBUG;
> `Config.EmitLevel` raises the event line. All metric names use the
> default `PerOpMetrics` schema; under `Config.MetricSchema:
> boeng.LabeledMetrics` every `<op>_*` family below collapses into
> `boeng_operation_duration_seconds{op="<op>",outcome=...}` and every
> `<event>_total` into `boeng_events_total{event="<event>"}`.

## How to read each scenario

- **Goal** — one sentence.
- **Code** — minimum reproducible snippet (link to the full example
  file when one exists).
- **Logs** — JSON lines the runtime emits, in order.
- **Trace tree** — operation hierarchy as it would appear in
  Tempo/Jaeger.
- **Metrics** — names of metric instruments the runtime emits.
- **Watch out for** — one specific mistake to avoid.

---

## 1. `Run` — happy path

**Goal:** wrap a function body in a single boeng operation.

**Code:**

```go
type User struct {
    UserID string
    Name   string
}
u := User{UserID: "u-1", Name: "Alice"}

_ = boeng.Run(ctx, "create_user", u, func(ctx context.Context) error {
    return nil
})
```

**Logs:**

```json
{"datetime":"2026-06-28T10:00:00.000Z","level":"DEBUG","msg":"create_user started","user_id":"u-1","name":"Alice","op":"create_user","service":"demo"}
{"datetime":"2026-06-28T10:00:00.001Z","level":"INFO","msg":"create_user completed","user_id":"u-1","duration_ms":1,"name":"Alice","op":"create_user","service":"demo"}
```

**Trace tree:**

```
create_user
```

**Metrics:**

- `create_user_total` ← +1
- `create_user_duration_ms` ← histogram observation
- `create_user_error_total` ← unchanged (no error)
- `create_user_panic_total` ← unchanged (no panic)

**Watch out for:** the `started` line is `DEBUG`. Production should
drop DEBUG to keep logs lean. Use the `INFO` completion line for "did
it happen?" dashboards.

---

## 2. `Run` — error path

**Goal:** capture an error returned by the function and surface it
on the operation.

**Code:**

```go
_ = boeng.Run(ctx, "charge_card", nil, func(ctx context.Context) error {
    return errors.New("amount must be positive")
})
```

**Logs:**

```json
{"datetime":"2026-06-28T10:00:00.000Z","level":"DEBUG","msg":"charge_card started","op":"charge_card","service":"demo"}
{"datetime":"2026-06-28T10:00:00.001Z","level":"ERROR","msg":"charge_card failed","duration_ms":1,"error":"amount must be positive","op":"charge_card","service":"demo"}
```

**Trace tree:**

```
charge_card  ✗ status=Error
```

**Metrics:**

- `charge_card_total` ← +1
- `charge_card_duration_ms` ← histogram observation
- `charge_card_error_total` ← +1
- `charge_card_panic_total` ← unchanged

**Watch out for:** the error is recorded *and* propagated. `Run`
returns the same `error` you returned from the closure — boeng does
not swallow it.

---

## 3. `Run` — panic recovery

**Goal:** ensure a panic inside the operation is captured, recorded,
**and re-raised** so the program unwinds normally.

**Code:**

```go
defer func() { _ = recover() }() // outer recover for the demo

_ = boeng.Run(ctx, "publish", nil, func(ctx context.Context) error {
    panic("kaboom")
})
```

**Logs:**

```json
{"datetime":"2026-06-28T10:00:00.000Z","level":"DEBUG","msg":"publish started","op":"publish","service":"demo"}
{"datetime":"2026-06-28T10:00:00.001Z","level":"ERROR","msg":"publish failed","duration_ms":0,"error":"panic: kaboom","op":"publish","service":"demo"}
```

The panic is then re-raised — your usual unwind continues.

**Trace tree:**

```
publish  ✗ status=Error (panic)
```

**Metrics:**

- `publish_total` ← +1
- `publish_duration_ms` ← histogram observation
- `publish_error_total` ← +1
- `publish_panic_total` ← +1

**Watch out for:** boeng records and re-raises. **Do not** wrap your
own `defer recover()` inside the closure to "be safe" — it would
swallow the panic the runtime relies on. If you need a recover for
graceful shutdown, place it *outside* `boeng.Run`.

---

## 4. Nested operations — `EnterCtx` + `Step`

**Goal:** decompose one logical operation into named phases so each
phase produces its own span, log line, and per-step metric family.

**Code:**

```go
_, op := boeng.EnterCtx(ctx, "create_user", nil)
defer op.Close()

_ = op.Step("validate",  func() error { return nil })
_ = op.Step("insert_db", func() error { return nil })
```

**Logs:**

```json
{"datetime":"2026-06-28T10:00:00.000Z","level":"DEBUG","msg":"create_user started","op":"create_user","service":"demo"}
{"datetime":"2026-06-28T10:00:00.001Z","level":"DEBUG","msg":"validate started","op":"validate","service":"demo"}
{"datetime":"2026-06-28T10:00:00.002Z","level":"INFO","msg":"validate completed","duration_ms":1,"op":"validate","service":"demo"}
{"datetime":"2026-06-28T10:00:00.003Z","level":"DEBUG","msg":"insert_db started","op":"insert_db","service":"demo"}
{"datetime":"2026-06-28T10:00:00.004Z","level":"INFO","msg":"insert_db completed","duration_ms":1,"op":"insert_db","service":"demo"}
{"datetime":"2026-06-28T10:00:00.005Z","level":"INFO","msg":"create_user completed","duration_ms":5,"op":"create_user","service":"demo"}
```

**Trace tree:**

```
create_user
├── validate
└── insert_db
```

**Metrics (each step has its own family):**

- `create_user_total`, `create_user_duration_ms`
- `validate_total`, `validate_duration_ms`
- `insert_db_total`, `insert_db_duration_ms`

**Watch out for:** step completions appear in the log stream
*before* the parent's completion (innermost defer runs first). That
ordering is part of guarantee G5 (correlation) and is asserted by
`TestEnterCtxChildSpan` in `boeng/verify_test.go`.

---

## 5. `Emit` — domain event inside an operation

**Goal:** record a named event mid-operation. Each `Emit` produces
exactly one log line, one span event, and one counter increment —
**not** a new operation, **not** a span of its own.

**Code:**

```go
_ = boeng.Run(ctx, "save_user", nil, func(ctx context.Context) error {
    boeng.Emit(ctx, "row_inserted", map[string]any{"row_id": 42})
    return nil
})
```

**Logs:**

```json
{"datetime":"2026-06-28T10:00:00.000Z","level":"DEBUG","msg":"save_user started","op":"save_user","service":"demo"}
{"datetime":"2026-06-28T10:00:00.001Z","level":"INFO","msg":"row_inserted","event":"row_inserted","op":"save_user","row_id":42,"service":"demo"}
{"datetime":"2026-06-28T10:00:00.002Z","level":"INFO","msg":"save_user completed","duration_ms":2,"op":"save_user","service":"demo"}
```

**Trace tree:** (events are markers, not children)

```
save_user
  • event "row_inserted" (row_id=42)
```

**Metrics:**

- `save_user_total`, `save_user_duration_ms`
- `row_inserted_total` ← +1 (independent of `save_user`)

**Watch out for:** use `Emit` for things that "happened" inside the
op, not for things that "took time". For things that take time, use
`op.Step` so you get a duration histogram for free.

---

## 6. `Loggable` fallback — auto-masking via reflection

**Goal:** show what happens when a struct does NOT implement
`LogFields()` — boeng falls back to reflection over exported fields,
auto-masking keys whose names suggest sensitive data.

**Code:**

```go
type User struct {
    UserID string
    Email  string
    Token  string
}
u := User{UserID: "u-1", Email: "john@doe.com", Token: "secret-xyz"}

_ = boeng.Run(ctx, "load_user", u, func(ctx context.Context) error {
    return nil
})
```

**Logs:**

```json
{"datetime":"2026-06-28T10:00:00.000Z","level":"INFO","msg":"load_user completed","user_id":"u-1","duration_ms":0,"email":"j**n@doe.com","op":"load_user","service":"demo","token":"s********z"}
```

**Trace tree:**

```
load_user
```

**Metrics:**

- `load_user_total`, `load_user_duration_ms`

**Watch out for:**

- The reflection fallback converts names to snake_case (`UserID` →
  `user_id`) and skips zero values.
- It auto-masks keys containing `email`, `token`, `secret`,
  `password`, `phone`, `apikey`, `auth`, `ssn`, `credit_card`.
- For *anything* domain-specific, implement `LogFields()` on the
  struct yourself — the reflection masker is a fail-safe, not a
  policy. `examples/boeng_ctx/main.go` shows the Loggable pattern.

---

## 7. Cardinality guard — high-cardinality fields stay in logs but never become metric labels

**Goal:** prove the documented promise that subject fields like
`user_id` and `order_id` go into log lines but are dropped at metric
emission time so Prometheus cardinality stays bounded.

**Code:**

```go
boeng.Init(boeng.Config{
    Service:      "demo",
    Env:          "prod",
    MetricLabels: []string{"tier"}, // ONLY allowlisted keys become labels
})

// subject has both a low-cardinality "tier" and high-cardinality IDs
subj := map[string]any{
    "user_id":  "u-99999",
    "order_id": "ord-12345",
    "tier":     "premium",
}
```

**Outcome:**

| Where | `tier` | `service` / `env` | `user_id` | `order_id` |
| ----- | ------ | ----------------- | --------- | ---------- |
| Log line (top-level JSON field) | ✓ | ✓ | ✓ | ✓ |
| Span attribute | ✓ | ✓ | ✓ | ✓ |
| **Metric label** | ✓ | ✓ (always) | ✗ filtered | ✗ filtered |

The cardinality guard is **fail-closed**: a key that isn't named in
`Config.MetricLabels` is silently dropped at metric-emission time,
even if `LogFields()` or reflection produced it. The pure helper that
makes the decision is verified by
[`TestMetricCardinalityGuard`](../boeng/verify_test.go) and the
runtime check by `TestWalkthrough_CardinalityGuard` in
[`walkthrough_test.go`](../boeng/walkthrough_test.go).

**Watch out for:** **never** add `user_id`, `order_id`, `request_id`,
`email`, IP, or any free-form text to `MetricLabels`. Each unique
value creates one Prometheus time-series; the wrong label can take
down a metrics backend.

---

## 8. HTTP propagation — `traceparent` across services

In-process scenarios above run without an OTel collector. The HTTP
propagation case is verified separately because it requires two
endpoints. The how-to — which adapter injects and extracts, what your
code must do (thread `ctx`, same backend, the sampled flag) and how to
inject by hand for transports without an adapter — is
["Tracing across services"](../boeng/README.md#tracing-across-services)
in the package README. See
[`boeng/http/propagation_test.go`](../boeng/http/propagation_test.go),
which asserts:

- `boenghttp.Transport` injects a W3C `traceparent` header into every
  outgoing request.
- `boenghttp.Middleware` extracts it on the server side and the
  resulting `ctx` is the one passed to your handler — so subsequent
  `boeng.Run` calls under it nest under the caller's span.

The chain test [`boeng/integration/chain_test.go`](../boeng/integration/chain_test.go)
(build tag `integration`) follows one request through Gin → RabbitMQ →
Redis → Mongo and asserts every hop produces a boeng operation.

---

## Where to look next

- [`../boeng/README.md`](../boeng/README.md) — full API surface and
  runtime guarantees G1–G7.
- [`testing.md`](./testing.md) — every test layer that gates this
  documentation.
- [`ai/guardrails.md`](./ai/guardrails.md) — explicit DO / DO NOT
  rules when writing boeng code.

> Verified against `6148b99` · 2026-09-18

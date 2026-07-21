# boeng

**boeng is an Operation Runtime for Go.**

Developers describe business operations — not observability. From a single
operation declaration, boeng's runtime automatically generates structured
logs, distributed traces, operation metrics, panic recovery, and
correlation, and exports them through a configurable pipeline. Business
code stays focused on business intent.

## What you write

```go
err := boeng.Run(ctx, "create_user", usr, func(ctx context.Context) error {
    return repo.Create(ctx, usr)
})
```

That's it. No span. No counter. No log statement. No `ctx` plumbing for
trace propagation. Application code never sees the word "OpenTelemetry."

## Architecture: Runtime + Pipeline

```
Business code
    │ describes
    ▼
Operation  ── name + subject + outcome
    │ dispatched into
    ▼
Pipeline
    ├── Logger     (structured log entries)
    ├── Trace      (spans, correlation, propagation)
    ├── Metric     (per-op counters + duration histograms)
    ├── Recover    (panic capture + re-raise)
    ├── Sampling   (configurable, post-MVP)
    ├── Security   (PII masking)
    ├── Exporter   (OTLP, optional)
    └── Sink       (stdout JSON, Loki, file, alerts)
```

Future additions to the pipeline — audit, profiling, anomaly detection —
do not change the public API. Application code stays unchanged.

## Four verbs

| Verb         | Purpose                                                 |
| ------------ | ------------------------------------------------------- |
| `Init`       | Process startup, once                                   |
| `Run`        | **Wrap a function** in an operation                     |
| `Enter` / `EnterCtx` | **Open an operation from inside a function**    |
| `Emit`       | Standalone domain event (no enclosing operation)        |

### Run vs Enter — intent, not technology

This is the question everyone asks first. The answer is about *what shape
of code you're instrumenting*, not about ctx or legacy:

- **Use `Run` when you want to wrap an entire function body in one
  operation.** The closure body is the operation; observability happens
  at the boundaries automatically.
- **Use `Enter` (or `EnterCtx`) when you're already inside a function and
  you want to open an operation by hand**, optionally composing it from
  multiple `Step`s, logging mid-flow, or wiring a `defer op.Close()` in
  legacy code that you don't want to refactor.

If your function reads naturally as one body → `Run`. If it reads as a
sequence of phases or you need a handle for `op.Log` / `op.Emit` /
`op.Step` → `Enter`.

## Operation handle methods

Everything stays on the operation handle returned by `Enter` / `EnterCtx`:

```go
op.Step(name, fn)          // child operation through the full pipeline
op.Log(msg, subjects...)   // ad-hoc log line in the op's scope
op.Emit(name, subject)     // named domain event in the op's scope
op.Fail(err)               // mark failed; Close still required
op.Success()               // explicit happy-path marker (optional)
op.Close() / op.CloseWith(&err)
op.Context()               // pass downstream for nesting
```

## Three usage modes

### 1. Greenfield — closure (`Run`)

```go
err := boeng.Run(ctx, "create_user", usr, func(ctx context.Context) error {
    return repo.Create(ctx, usr)
})
```

For functions returning `(T, error)`:

```go
user, err := boeng.RunR(ctx, "load_user", id, func(ctx context.Context) (User, error) {
    return repo.Load(ctx, id)
})
```

### 2. Phased — Enter + Step

When an operation has distinct phases worth measuring individually:

```go
func createUser(ctx context.Context, usr User) (err error) {
    ctx, op := boeng.EnterCtx(ctx, "create_user", usr)
    defer op.CloseWith(&err)

    if err = op.Step("validate", func() error { return validate(usr) }); err != nil {
        return err
    }
    if err = op.Step("insert_db", func() error { return repo.Insert(ctx, usr) }); err != nil {
        return err
    }
    return op.Step("publish", func() error { return publisher.Send(ctx, usr) })
}
```

What you get for free, per Step:

- Child span attached to the parent operation's trace
- Per-step metrics: `validate_total`, `validate_duration_ms`,
  `validate_error_total`, `validate_panic_total`
- Start/completion log lines with `op` field naming the step
- Panic recovery — the step's panic is recorded against the step's span
  and metrics, the parent operation is marked failed, then the panic
  re-raises so unwinding continues
- Error cascade — a step returning a non-nil error marks the parent op
  as failed even if the caller forgets to propagate the return value

### 3. Legacy — Enter without ctx

For codebases that don't thread `context.Context`. Function signatures
stay exactly as they are:

```go
func saveUser(usr User) {
    op := boeng.Enter("save_user", usr)
    defer op.Close()

    _ = op.Step("validate", func() error { return validate(usr) })
    _ = op.Step("insert_db", func() error { return insertDB(usr) })
    _ = op.Step("publish", func() error { return publish(usr) })
}
```

This is the migration pattern boeng was specifically designed for.
OpenTelemetry's SDK has no equivalent — you can adopt boeng into a legacy
codebase without a ctx refactor first.

## Subjects (the business data an op acts on)

Pass a struct, `*struct`, `map[string]any`, or `nil`. Resolution priority:

1. `nil` → no fields attached
2. Type implements `boeng.Loggable` → use `LogFields()`
3. `map[string]any` → used as-is
4. Struct → reflected exported fields

### Loggable (recommended for domain types)

Implement `LogFields()` on domain structs so observability lives in the
domain layer — no struct tags, no scattered masking logic, no
observability imports leaking into the model.

```go
type User struct {
    UserID string
    Email  string
    Token  string  // never returned, never logged
}

func (u User) LogFields() map[string]any {
    return map[string]any{
        "user_id": u.UserID,
        "email":   boeng.MaskEmail(u.Email),
    }
}
```

### Reflection fallback

If a subject doesn't implement `Loggable`, boeng reflects over exported
fields:

- Names convert to snake_case (`UserID` → `user_id`, `HTTPStatus` →
  `http_status`)
- `json:"..."` tag's first token overrides the snake_case name
- Zero values are skipped (toggle via `Config.IncludeZeroFields`)
- Keys containing `password`, `token`, `secret`, `email`, `phone`,
  `apikey`, `auth`, `ssn`, `credit_card` are auto-masked
- Unexported fields are ignored

## Setup

```go
obs := boeng.Init(boeng.Config{
    Service:      "checkout",
    Version:      "1.4.0",
    Env:          "prod",
    OTel:         "otel-collector:4317",
    Loki:         "",
    Async:        true,
    Level:        boeng.InfoLevel,
    MetricLabels: []string{"user_type", "payment_channel"},
})
defer obs.Close()
```

Call `Init` once at process start. `Close` on shutdown flushes async
batches and shuts down the OTel exporter. Calling `Init` twice is safe —
the previous instance is closed first.

If `OTel` is empty or the collector is unreachable, boeng runs purely
against the configured sinks (stdout JSON by default). Application code
is identical either way; nothing panics.

## Metrics

For every operation (and every Step), boeng emits four metrics named
after the **sanitized operation name**:

| Metric                | Type      | When                                       |
| --------------------- | --------- | ------------------------------------------ |
| `<op>_total`          | counter   | every completion                           |
| `<op>_duration_ms`    | histogram | every completion (unit: ms)                |
| `<op>_error_total`    | counter   | op returned non-nil error or panicked      |
| `<op>_panic_total`    | counter   | op panicked (also bumps `_error_total`)    |

Names sanitize to Prometheus rules: `"GET /users/:id"` → `get_users_id`.

`boeng.Emit(...)` (or `op.Emit(...)`) emits `<event>_total`.

### Cardinality is fail-closed

By default, only `service` and `env` are attached as metric labels.
Subject fields are NOT promoted automatically. To allow specific
low-cardinality keys through:

```go
MetricLabels: []string{"user_type", "payment_channel", "tier", "region"}
```

Keys not in `MetricLabels` are silently dropped at metric-emission time
even if `LogFields()` or reflection produced them. The fields still show
up in logs (where cardinality is fine) and on spans (where sampling
absorbs the cost) — the allowlist only governs metric labels.

**Never** put `user_id`, `order_id`, `request_id`, `email`, free-form
text, or IPs in `MetricLabels`. Each unique value creates a Prometheus
time-series; a single ID column can take down the metrics backend.

## Output

Every log entry is a single JSON line on stdout (and any extra sinks):

```json
{
  "datetime": "2026-06-27T12:34:56.789012345+07:00",
  "level": "INFO",
  "msg": "create_user completed",
  "service": "checkout",
  "env": "prod",
  "op": "create_user",
  "user_id": "u-123",
  "email": "j**n@doe.com",
  "trace_id": "abc...",
  "span_id": "def...",
  "duration_ms": 17
}
```

Errors flip the level to `ERROR` and add an `"error"` field.

## Local stack

The full observability stack (OTel Collector → Tempo + Prometheus +
Grafana + Loki + Jaeger) is defined under `compose/` at the module root:

```bash
cd ../compose
docker compose --profile full up -d
```

Then point `Config.OTel` at `localhost:4317`. Grafana lives at
`localhost:3000`, Jaeger at `localhost:16686`. The smaller `mini` and
`file` profiles are useful for local smoke-testing.

## Why "Operation Runtime" and not "logger"

`slog`, `zap`, and the OpenTelemetry SDK all live one layer too low for
application code. They give you primitives — log records, spans,
counters, histograms — and ask you to compose them correctly at every
call site. The result is observability code that's louder than the
business logic it instruments.

boeng inverts that. The unit of instrumentation is an *operation*, not a
log line. From one declaration the runtime derives logs, traces,
metrics, correlation, panic handling, and error recording — all
consistent, all named consistently, all cardinality-safe, all
dispatched through the same pipeline. Application code stays focused on
business intent; observability becomes the runtime's responsibility.

## Runtime Guarantees

These are the seven user-facing promises boeng makes. Every promise is
backed by a named test in `boeng/guarantees_test.go`. If any of these
fails, a documented promise has broken.

| # | Guarantee | Test |
| - | --------- | ---- |
| G1 | Every operation produces a structured log line on completion | `TestGuarantee_G1_StructuredCompletionLog` |
| G2 | Every operation reports `duration_ms` | `TestGuarantee_G2_DurationReported` |
| G3 | Metric labels respect the cardinality allowlist (fail-closed) | `TestGuarantee_G3_MetricLabelsAreCardinalitySafe` |
| G4 | Every panic is captured, recorded, AND re-raised | `TestGuarantee_G4_PanicCapturedAndReraised` |
| G5 | Every child operation preserves correlation through context | `TestGuarantee_G5_ChildOpPreservesCorrelation` |
| G6 | Every adapter accepts a parent ctx and produces a child operation | `TestGuarantee_G6_AdapterAcceptsParentCtx` |
| G7 | Exporter failure degrades gracefully (no panic, sinks keep working) | `TestGuarantee_G7_ExporterFailureDegradesGracefully` |

## Test pyramid

| Layer | Location | What it proves |
| ----- | -------- | -------------- |
| Unit | `boeng/*_test.go` (`fields_test`, `enter_test`, `run_test`, `metrics_test`) | Individual functions behave correctly. |
| Verification | `boeng/verify_test.go` | End-to-end user promises (`TestRunSuccess`, `TestRunError`, `TestRunPanic`, `TestEnterLegacy`, `TestEnterCtxChildSpan`, `TestMetricCardinalityGuard`, `TestOTLPFallback`). |
| Runtime Guarantees | `boeng/guarantees_test.go` | The 7 published promises above. |
| Golden Trace | `boeng/golden_test.go` + `boeng/testdata/golden_*.json` | A canonical `Run → Step → Emit` tree produces an exact, redacted log stream. Regenerate with `go test -run TestGoldenTrace ./boeng/ -update`. |
| API Freeze | `boeng/api_freeze_test.go` | Compile-time enforcement that no public symbol disappears between releases. |
| Adapter Compliance | `boeng/http/propagation_test.go`, `boeng/{gin,mongo,redis,rabbit}/*_test.go` | Each adapter accepts ctx, opens an op, and propagates trace context across the medium it instruments. |
| Integration | `boeng/{mongo,redis,rabbit}/integration_test.go` (build tag `integration`) | Each adapter actually drives its real backend and verifies a boeng op fires for the operation. Skipped when the backend isn't reachable. |
| End-to-end chain | `boeng/integration/chain_test.go` (build tag `integration`) | One HTTP request fans out through Rabbit → Redis → Mongo; the test asserts every hop produces a boeng op. |
| Performance | `benchmarks/log_bench_test.go` | `BenchmarkRun`, `BenchmarkRunWithSubject`, `BenchmarkRunErrorPath`, `BenchmarkEnterStep` (`go test -bench ./benchmarks`). |

Run the integration layers locally with:

```bash
docker compose --profile full up -d   # mongo, redis, rabbit + observability stack
go test -tags integration ./...
```

CI runs both layers automatically — see `.github/workflows/test.yml`.

## API stability

The public surface above is what users will see in v1.0. Future work
adds new pipeline stages (audit, profiling, sampling tuning) without
changing the four verbs or the operation-handle methods. The
`TestPublicAPI` freeze test enforces that no exported identifier
disappears without an intentional, documented major-version change.

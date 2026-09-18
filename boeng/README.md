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
    OTel:         "http://otel-collector:4317",
    Loki:         "",
    Async:        true,
    Level:        boeng.InfoLevel,
    QuietOps:     true,            // "<op> completed" at DEBUG; failures stay ERROR
    EmitLevel:    boeng.InfoLevel, // level of Emit event lines (zero = INFO)
    MetricLabels: []string{"user_type", "payment_channel"},
})
defer obs.Close()
```

Call `Init` once at process start. `Close` on shutdown flushes async
batches and shuts down the OTel exporter. Calling `Init` twice is safe —
the previous instance is closed first.

An unreachable collector never panics and never blocks: logs keep
flowing to the configured sinks (stdout JSON by default) and spans still
carry valid ids. Where spans and metrics go when `OTel` is empty is
decided by the environment and by what the process already installed —
see the next two subsections. Application code is identical either way.

### Where the collector address comes from

`Config.OTel` follows the OTel exporter spec: it is a URL, and the scheme
decides transport security.

| `Config.OTel`                     | Transport                      |
| --------------------------------- | ------------------------------ |
| `http://otel-collector:4317`      | plaintext gRPC                 |
| `https://otel-collector:4317`     | TLS, system root certificates  |
| `otel-collector:4317` (pre-1.3)   | plaintext, unchanged behaviour |
| empty                             | see below                      |

With `Config.OTel` empty, boeng exports **only if** the deployment set
`OTEL_EXPORTER_OTLP_ENDPOINT` (or `_TRACES_ENDPOINT` / `_METRICS_ENDPOINT`).
boeng then passes no endpoint to the exporters and the SDK reads the
whole spec'd env set itself — endpoint, `OTEL_EXPORTER_OTLP_HEADERS`,
`_TIMEOUT`, `_CERTIFICATE`. That is the recommended production shape:
configure telemetry once, in the environment, the same way for every
service and language:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

Do not copy that env value into `Config.OTel` as well; one source is
enough, and a scheme-less IP form (`10.0.0.1:4317`) in the env makes the
SDK print a `parse url` warning per exporter at boot — the env value must
be a URL. `Config.Service` is required to export in either mode; without
it boeng never sends anonymous telemetry. With no endpoint anywhere,
boeng still opens spans (valid `trace_id` / `span_id`, W3C propagation)
but exports nothing and records no metrics.

### Living inside an existing OTel setup

boeng joins the process's OpenTelemetry configuration instead of
replacing it:

- **TracerProvider.** If the application already installed an SDK
  `TracerProvider` via `otel.SetTracerProvider` and `Config.OTel` is
  empty, boeng opens its spans on that provider — its resource
  attributes, its sampler, its exporter — and never shuts it down. An
  `OTEL_EXPORTER_OTLP_ENDPOINT` in the environment is taken to belong to
  that pipeline and does not make boeng build a second one. Only when
  nothing is installed does boeng build its own provider: exporting if
  the env names an endpoint, non-exporting otherwise. A non-empty
  `Config.OTel` always means "export through boeng" and wins.
- **MeterProvider.** boeng's per-op instruments are created on the global
  meter. If the application installed an SDK `MeterProvider`, they record
  into it and leave through the application's exporter; boeng never
  replaces that provider, and `MetricsHandler` then answers 503 with a
  line saying where the metrics went (a failing scrape is the honest
  signal). Only when no application provider exists does boeng install
  its own — when exporting, or on the first `MetricsHandler` call.
- **Propagator.** boeng installs `TraceContext + Baggage` only when the
  process has no propagator yet. A propagator you set first (for
  instance `TraceContext` alone) stays; the adapters read the global at
  call time.
- **Without `Init`.** `Run` / `Enter` still open spans on the global
  tracer, so a service that configures OpenTelemetry itself gets boeng
  operations in its traces with no boeng setup at all. Logs then go to
  the default stdout logger.
- **Dependencies.** Importing `boeng` links only the OTel SDK and the
  Prometheus client. Gin, Mongo, RabbitMQ and Redis code lives in the
  `boeng/gin`, `boeng/mongo`, `boeng/rabbit`, `boeng/redis` sub-packages
  and is linked only when imported (guarded by
  `TestCoreBoengLinksNoFrameworks`).

### Which lines you see at which level

| Line                        | Level                                   | Knob              |
| --------------------------- | --------------------------------------- | ----------------- |
| `<op> started`              | DEBUG                                   | —                 |
| `<op> completed`            | INFO, or DEBUG with `QuietOps: true`    | `Config.QuietOps` |
| `<op> failed`               | ERROR, always                           | —                 |
| `Emit` / `op.Emit` event    | INFO, or `Config.EmitLevel` if higher   | `Config.EmitLevel`|
| `op.Log`                    | INFO                                    | —                 |

`Config.Level` filters all of them. The recommended production shape is
`Level: InfoLevel, QuietOps: true`: one span + four metrics per
operation as before, events and manual lines visible, no INFO line per
operation. If a deployment must run at `WarnLevel`, set
`EmitLevel: boeng.WarnLevel` so per-request summary events survive the
filter — dashboards built from log-derived metrics go blank otherwise.
Emit is never written below INFO.

### Reserved keys

boeng owns these top-level JSON keys and metric labels. Do not put them
in `LogFields()` or in a subject struct; rename yours (`req_service`,
`client_env`, `stage`, …):

| Key           | Set by                                    | On subject collision                                   |
| ------------- | ----------------------------------------- | ------------------------------------------------------ |
| `service`     | `Config.Service`                          | log: subject value wins; metric label: `Config` wins   |
| `env`         | `Config.Env`                              | log: subject value wins; metric label: `Config` wins   |
| `op`          | `Run` / `Enter` / `Step` name             | overwritten by boeng                                   |
| `event`       | `Emit` name                               | overwritten by boeng                                   |
| `duration_ms` | completion line                           | overwritten by boeng                                   |
| `trace_id`, `span_id` | active span                       | overwritten by boeng                                   |
| `error`       | failure line                              | overwritten by boeng                                   |

The `service`/`env` split (log vs metric) is the one place a collision
is not resolved uniformly — treat both keys as strictly reserved.

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

### Names as Prometheus sees them

The instruments carry no OTel unit, so neither the in-process exporter
nor a collector's Prometheus translation appends a unit suffix. What you
write in PromQL is exactly:

| Instrument           | Prometheus series                                                     |
| -------------------- | --------------------------------------------------------------------- |
| `<op>_total`         | `<op>_total`                                                          |
| `<op>_duration_ms`   | `<op>_duration_ms_bucket`, `<op>_duration_ms_sum`, `<op>_duration_ms_count` |
| `<op>_error_total`   | `<op>_error_total`                                                    |
| `<op>_panic_total`   | `<op>_panic_total`                                                    |
| `<event>_total`      | `<event>_total`                                                       |

Every series carries `service` and `env` labels plus any allowlisted
`MetricLabels`; the OTel Prometheus exporter adds `otel_scope_name="boeng"`.
Releases before 1.3.0 declared the histogram with unit `ms`, which
surfaced as `<op>_duration_ms_milliseconds_bucket` — update dashboards
written against that name.

### Labeled schema (`Config.MetricSchema`)

The per-op names above are the v1.x default (`PerOpMetrics`). They make
`sum by (op)` impossible — each operation is its own metric. Opt in to
one fixed family with the operation as a label:

```go
boeng.Init(boeng.Config{Service: "checkout", MetricSchema: boeng.LabeledMetrics})
```

| Instrument (OTel)          | Unit | Prometheus series                                        | Labels                              |
| -------------------------- | ---- | -------------------------------------------------------- | ----------------------------------- |
| `boeng.operation.duration` | s    | `boeng_operation_duration_seconds_{bucket,sum,count}`    | `op`, `outcome`, `service`, `env`, allowlist |
| `boeng.events`             | —    | `boeng_events_total`                                     | `event`, `service`, `env`, allowlist |

`outcome` is `ok`, `error` or `panic`; the histogram's `_count` is the
operation total, so there is no separate `_total` counter. Buckets are
1 ms … 10 s. `op` is the sanitized operation name (same value as the
legacy metric stem) and is bounded by the same 512-distinct-names cap,
past which it becomes `op="overflow"`.

```promql
sum by (op) (rate(boeng_operation_duration_seconds_count[5m]))
sum by (op) (rate(boeng_operation_duration_seconds_count{outcome!="ok"}[5m]))
histogram_quantile(0.95, sum by (op, le) (rate(boeng_operation_duration_seconds_bucket[5m])))
```

`BothMetrics` emits both shapes while dashboards migrate. The default
flips to `LabeledMetrics` in the next major version.

### Pull-based `/metrics`

Metrics leave the process over OTLP when `Config.OTel` is set. If the
collector does not accept metrics (some Alloy / Collector pipelines are
traces-only), mount the in-process registry on your own HTTP server:

```go
mux.Handle("/metrics", obs.MetricsHandler())
```

The handler serves every series above in exposition format. With
`Config.OTel` empty, boeng records no metrics by default; the first
`MetricsHandler` call switches recording on, so the page is never
silently empty. boeng does not open a listener — bind, timeouts, and
auth stay with the application.

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

`trace_id` / `span_id` come from the innermost boeng operation in `ctx`.
When `ctx` carries no boeng operation but does carry a valid OTel span
opened by something else (otelhttp, otelgin, a gRPC interceptor, your
own middleware), `boeng.L(ctx)` stamps that span's ids instead, so lines
written under third-party spans still join the trace.

## Tracing across services

boeng carries trace context between processes with W3C Trace Context
(`traceparent` / `tracestate` headers) — the same format every OTel SDK
speaks, so the other side does not need boeng, or Go. Nothing to
configure: `Init` makes sure a propagator exists (and keeps one the
process already set).

| Hop                 | Caller side                                   | Callee side                                                        |
| ------------------- | --------------------------------------------- | ------------------------------------------------------------------ |
| HTTP                | `&http.Client{Transport: boenghttp.Transport(nil)}` injects the header | `boenghttp.Middleware(mux)` or `boenggin.Middleware()` extracts it and opens the request op as a child |
| RabbitMQ            | `boengrabbit.Publish(ctx, ch, exchange, key, msg)` writes it into AMQP headers | `boengrabbit.Consume(queue, handler)` reads it and opens the delivery op as a child |
| Same process        | pass the `ctx` that `Run` / `EnterCtx` hands you | —                                                                  |

Minimal two-service shape (see [`examples/http`](../examples/http)):

```go
// service A — caller
client := &http.Client{Transport: boenghttp.Transport(nil)}
err := boeng.Run(ctx, "checkout", order, func(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, body) // ctx carries the span
    _, err := client.Do(req)                                               // header injected here
    return err
})

// service B — callee
mux.HandleFunc("/charge", func(w http.ResponseWriter, r *http.Request) {
    _ = boeng.Run(r.Context(), "charge", nil, func(ctx context.Context) error { // child of A's span
        return chargeCard(ctx)
    })
})
srv := &http.Server{Addr: ":8081", Handler: boenghttp.Middleware(mux)}
```

Both services log the same `trace_id`; with a shared backend the two
spans render as one trace.

What has to be true on your side:

1. **Thread `ctx` all the way.** A `context.Background()` created in the
   middle of a request starts a new, unrelated trace. This is the usual
   cause of "the trace stops at service B".
2. **Use an adapter at every boundary, or inject by hand.** For a
   transport without a boeng adapter (gRPC, Kafka, SQS, …) use that
   transport's OTel instrumentation, or inject/extract yourself with
   the global propagator — it is the same propagator boeng uses:
   ```go
   otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
   ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
   ```
3. **Export both sides to the same backend.** Propagation makes the ids
   match; only export makes the picture. A service with no endpoint
   still passes `traceparent` on, but its own spans exist only as
   `trace_id` / `span_id` in its logs.
4. **Mind the sampled flag.** With no collector boeng samples nothing
   (`sampled=0` in the outgoing `traceparent`). A downstream boeng
   service in export mode records anyway (AlwaysSample), but a plain
   OTel service with the default parent-based sampler will honour the
   flag and drop the trace. In production give the *first* service an
   endpoint too; do not rely on downstream services to record for it.

Verified by [`boeng/http/propagation_test.go`](./http/propagation_test.go)
(two real endpoints, one `trace_id`) and, across Gin → RabbitMQ →
Redis → Mongo, by [`boeng/integration/chain_test.go`](./integration/chain_test.go).

## Local stack

The full observability stack (OTel Collector → Tempo + Prometheus +
Grafana + Loki + Jaeger) is defined under `compose/` at the module root:

```bash
cd ../compose
docker compose --profile full up -d
```

Then point `Config.OTel` at `http://localhost:4317`. Grafana lives at
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
| Unit | `boeng/*_test.go` (`fields_test`, `enter_test`, `run_test`, `metrics_test`, `levels_test`, `l_spancontext_test`) | Individual functions behave correctly; `QuietOps` / `EmitLevel`; `L(ctx)` correlation from third-party spans. |
| Metric surface | `boeng/metrics_handler_test.go`, `boeng/metrics_labeled_test.go` | `/metrics` serves the registry with the documented names (no unit suffix); `LabeledMetrics` / `BothMetrics` shapes and the sanitized, capped `op` label. |
| OTel coexistence | `boeng/otel_env_test.go`, `boeng/propagator_test.go`, `boeng/tracer_adopt_test.go`, `boeng/meter_adopt_test.go`, `internal/adapter/outbound/otel/endpoint_test.go` | Endpoint URL → transport; env fallback; an application's TracerProvider / MeterProvider / propagator is adopted, never replaced or shut down; `Run` without `Init` uses the global tracer. |
| Dependency guard | `boeng/deps_guard_test.go` | Importing `boeng` alone links no web/database framework. |
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

> Verified against `6148b99` · 2026-09-18

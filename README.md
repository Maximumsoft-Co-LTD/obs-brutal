# obs-brutal

**An Operation Runtime for Go.**

`obs-brutal` ships a single public package — [`boeng`](./boeng) — that
lets application code describe **business operations** instead of
observability primitives. From one operation declaration the runtime
generates structured logs, distributed traces, per-operation metrics,
panic recovery, and correlation, and exports them through a configurable
pipeline.

```go
err := boeng.Run(ctx, "create_user", usr, func(ctx context.Context) error {
    return repo.Create(ctx, usr)
})
```

No `span.End()`. No `metric.Add()`. No `slog.InfoContext()`. No
OpenTelemetry import in your business code. Get logs, traces, metrics,
panic capture, and error recording from one line.

## Philosophy

> **Everything starts from an Operation.**

Application code should answer one question — "what operation am I
doing right now?" — and observability follows mechanically from that
answer. Logs, traces, metrics, duration, error capture, panic recovery,
and cross-process correlation are not separate concerns to wire up at
every call site; they are derivatives of the same Operation
declaration.

This is the entire design thesis. Every other choice in this repo
exists to keep it true:

- **Three verbs, not twelve concepts.** `Run` / `Enter` / `Emit` are
  the only words an application needs. `Tracer`, `Span`, `Meter`,
  `Counter`, `Histogram`, `Propagator`, `Exporter` are runtime
  implementation, not application vocabulary.
- **Adapters are boundary translators.** `boeng/http`, `boeng/rabbit`,
  `boeng/mongo`, `boeng/redis`, `boeng/gin` exist to turn an external
  signal (a request, an AMQP delivery, a DB command) into exactly one
  Operation. The runtime treats that Operation identically to one
  written by hand.
- **Defaults are correct.** Stdout JSON, async pipeline, panic
  recovery, W3C propagator — all on by default. The user is not
  expected to know enough about observability to assemble them.
- **The freeze test is the contract.** Public surface lives in one
  file (`boeng/api_freeze_test.go`). If it changes, semver does.

For the full architectural sketch see [`ARCHITECTURE.md`](./ARCHITECTURE.md).

## Non-goals

To keep the four-verb invariant honest, boeng explicitly does NOT
do the following — pull requests that violate any of these will be
declined regardless of how nice the implementation is:

- **Be a general-purpose logging library.** Use `slog` if you want
  one. boeng is an Operation Runtime; logs are a byproduct.
- **Re-implement OpenTelemetry.** boeng wraps the OTel SDK; it does
  not compete with it. Anything tracer-level (sampler internals,
  exporter wire format) lives in OTel.
- **Expose pipeline stages as a public API.** Future stages (audit,
  profiling, anomaly detection) will land behind the same four verbs.
  Application code never sees them.
- **Accept arbitrary user-built sinks via Config.** The cardinality and
  safety guarantees depend on the small set of sinks the runtime ships.
  Custom sinks are a v2 conversation.
- **Provide every framework adapter.** The five shipped adapters (gin,
  http, mongo, redis, rabbit) cover the common cases. Additional ones
  belong in separate modules so the main module's surface stays small.
- **Log levels beyond Debug/Info/Warn/Error/Fatal.** No `Trace`, no
  `Verbose`, no custom levels.
- **Accept high-cardinality fields as metric labels.** Cardinality is
  fail-closed by design; allowlist exists for low-cardinality keys only.

## Supported Go versions

| Go release | Status |
| ---------- | ------ |
| 1.25 | Minimum supported — the `go` directive in `go.mod`; CI runs against this |
| ≤ 1.24 | Unsupported — the module requires Go 1.25 (uses 1.25 features such as `clear()`) |

Bumping the minimum Go version is a breaking change per
[`COMPATIBILITY.md`](./COMPATIBILITY.md).

## Why it exists

`slog`, `zap`, and the OpenTelemetry SDK all live one layer too low for
application code. They give you primitives — log records, spans,
counters, histograms — and ask you to compose them correctly at every
call site. The result is observability code that's louder than the
business logic it instruments.

`boeng` inverts that. The unit of instrumentation is an *operation*, not
a log line. The runtime dispatches one operation declaration through a
pipeline of logger, tracer, meter, recover, security, sampling,
exporter, and sink stages, producing consistent output across all of
them.

## Product goals

boeng is judged against five user-facing outcomes, not against
code-coverage percentages. Every goal is backed by a test in this
repository.

| # | Goal                                                       | How it's verified |
| - | ---------------------------------------------------------- | ----------------- |
| P1 | Adding observability reduces business-code LOC by ≥80%     | `TestAITransformation_Fixtures` (each transform stays within a budget of 3–5 added lines) |
| P2 | A coding agent can add observability without reading docs  | `boeng/testdata/transform/` (canonical before/after pairs the agent must reproduce; pinned by `TestAITransformation_Fixtures`) |
| P3 | A legacy function adopts boeng without changing its signature | `TestMigration_SignatureUnchanged` + `TestMigration_BehaviorPreserved` |
| P4 | A new developer learns the API in under 10 minutes         | 4 verbs (`Run` / `Enter` / `Emit` / `Init`) + 8 operation-handle methods (`Step` / `Log` / `Emit` / `Fail` / `Success` / `Close` / `CloseWith` / `Context`). Each is a single English word. |
| P5 | Business code never imports OpenTelemetry or any logger    | `TestAITransformation_Fixtures` enforces a banned-imports list; `boeng/api_freeze_test.go` documents the entire public surface |

## Human effort benchmark

The same observability outcome — log start/success/error + duration +
metric + trace span — costs roughly an order of magnitude more code in
hand-rolled OpenTelemetry + slog than in boeng. Counts come from the
`testdata/transform/` fixtures (see `TestAITransformation_Fixtures`).

| Task                              | OpenTelemetry + slog | boeng |
| --------------------------------- | -------------------- | ----- |
| Open + close an operation         | ~10 LOC              | 2 LOC (`Enter` + `defer Close`) |
| Capture error on op               | ~3 LOC (`RecordError`, `SetStatus`) | 0 LOC (built into `Close`) |
| Record duration histogram         | ~5 LOC (timer + Add) | 0 LOC (built in) |
| Recover panic on op               | ~7 LOC (`defer recover`, record, re-panic) | 0 LOC (built in) |
| Increment per-op counter          | ~3 LOC               | 0 LOC (built in) |
| Propagate W3C trace across HTTP   | ~6 LOC (`propagation.Inject` / `Extract`) | 0 LOC (`boenghttp.Transport` / `Middleware`) |
| **Total to instrument one op**    | **≈ 34 LOC**         | **2 LOC** |

Caveat: hand-rolled OTel numbers assume each call site writes idiomatic
code; library wrappers can shorten this, but every wrapper is one more
indirection users have to learn. boeng's deal is "two lines, one mental
model."

## Cognitive load

What an application developer must keep in their head to add
observability:

| Library                  | Concepts a developer must learn | Example list |
| ------------------------ | ------------------------------- | ------------ |
| OpenTelemetry SDK + slog | ~12                             | Tracer, Span, Context, Propagator, Exporter, Meter, Counter, Histogram, Attribute, Status, RecordError, Logger |
| boeng                    | 3 + 8 op-handle methods         | Operation (the noun); Run / Enter / Emit (the verbs); op.Step / Log / Emit / Fail / Success / Close / CloseWith / Context |

The boeng method names read as English sentences — `op.Step("validate", …)`,
`op.Fail(err)`, `op.Close()` — which is the point.

## AI token budget

When a coding agent is asked "add observability to this function," the
prompt + expected diff is roughly an order of magnitude smaller with
boeng than with raw OpenTelemetry + slog. The exact tokens depend on
the model's tokenizer, but the fixture pairs in
`boeng/testdata/transform/` make the cost concrete:

| Transformation       | Diff size (boeng) | Equivalent OTel diff (rough) | Ratio |
| -------------------- | ----------------- | ---------------------------- | ----- |
| Plain function       | +3 lines           | +28 lines                    | ~9× |
| Error-returning      | +4 lines           | +32 lines                    | ~8× |
| Panic recovery       | +5 lines           | +40 lines                    | ~8× |
| HTTP handler         | +3 lines           | +24 lines                    | ~8× |

Smaller diff = smaller LLM context window = lower token cost per
instrumentation request, and more importantly, fewer chances for the
agent to drift across SDK call sites in inconsistent ways.

## boeng vs OpenTelemetry + slog

|                          | OpenTelemetry SDK + slog              | boeng                                       |
| ------------------------ | -------------------------------------- | ------------------------------------------- |
| Open an operation        | 10–20 lines (tracer.Start, defer End, meter.Add, slog.Info, recover loop) | 1 line — `boeng.Run` / `boeng.Enter` |
| Log + Trace + Metric     | Write each one at every call site       | Automatic from one operation declaration    |
| Legacy migration         | Refactor every function to thread ctx   | `op := boeng.Enter(...); defer op.Close()` — signature unchanged |
| Context propagation      | You wire `propagation.Extract`/`Inject` everywhere | Runtime + adapters handle it (HTTP, RabbitMQ) |
| Panic recovery           | You write `defer func(){ if r:=recover(); … }` per scope | Built into every `Run` / `Step` / `Close`   |
| AI-assistant ergonomics  | LLMs scatter SDK calls inconsistently   | One concept (`operation`), one verb set     |

Honest performance (`go test -bench=Comparison -benchmem ./benchmarks/`,
Apple M2, sink-to-discard):

| Path                 | ns/op    | allocs | B/op  | What it includes                          |
| -------------------- | -------- | ------ | ----- | ----------------------------------------- |
| `log.Printf`         | ~70      | 3      | 48    | one unstructured line                     |
| `slog.LogAttrs`      | ~520     | 0      | 0     | one structured JSON line                  |
| `boeng.Emit`         | ~980     | 21     | 1664  | + event counter, log entry, span event    |
| `boeng.Run`          | ~2.4 µs  | 48     | 4136  | + span open/close, duration histogram, panic recovery, per-op counters |

The cost difference vs `slog` is what buys you trace + metric + panic
correlation. If you'd otherwise write those by hand, boeng is the
better deal; if you only need a bare log line, use `slog`.

## Quickstart

```bash
go get github.com/Maximumsoft-Co-LTD/obs-brutal/boeng
```

```go
package main

import (
    "context"

    "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

type User struct{ ID, Email string }

func (u User) LogFields() map[string]any {
    return map[string]any{
        "user_id": u.ID,
        "email":   boeng.MaskEmail(u.Email),
    }
}

func main() {
    defer boeng.Init(boeng.Config{Service: "demo", Env: "dev"}).Close()

    ctx := context.Background()
    _ = boeng.Run(ctx, "create_user", User{ID: "u-1", Email: "john@doe.com"},
        func(ctx context.Context) error {
            return nil
        })
}
```

Output:

```json
{"datetime":"2026-06-27T22:00:40.720Z","level":"INFO","msg":"create_user completed",
 "service":"demo","env":"dev","op":"create_user","user_id":"u-1",
 "email":"j**n@doe.com","duration_ms":0}
```

For trace + metrics, point `Config.OTel` at any OTLP gRPC collector. If
the collector is unreachable, boeng silently falls back to stdout JSON —
nothing panics.

## The four verbs

| Verb         | Purpose                                                 |
| ------------ | ------------------------------------------------------- |
| `Init`       | Process startup, once                                   |
| `Run`        | Wrap a function body in one operation                   |
| `Enter` / `EnterCtx` | Open an operation from inside a function        |
| `Emit`       | Standalone domain event (no enclosing operation)        |

Everything else lives on the operation handle:

```go
op.Step(name, fn)          // child operation through the full pipeline
op.Log(msg, subjects...)   // ad-hoc log inside the op's scope
op.Emit(name, subject)     // domain event inside the op's scope
op.Fail(err) / op.Success()
op.Close() / op.CloseWith(&err)
op.Context()
```

See [`boeng/README.md`](./boeng/README.md) for full API documentation,
the Loggable contract, reflection rules, cardinality guard, and the
"why Operation Runtime and not logger" deep dive.

## Examples

Core usage (no backend required):

```bash
go run ./examples/boeng         # greenfield: Run + closure
go run ./examples/boeng_ctx     # phased: EnterCtx + op.Step
go run ./examples/boeng_legacy  # legacy: Enter without ctx
go run ./examples/cron          # periodic worker: Run + Step per tick
go run ./examples/cli greet Alice   # CLI: RunR with exit code
```

Framework adapters (each requires the relevant backend running):

```bash
go run ./examples/gin     # Gin middleware
go run ./examples/http    # net/http server + client RoundTripper
go run ./examples/mongo   # MongoDB CommandMonitor
go run ./examples/redis   # go-redis Hook
go run ./examples/rabbit  # RabbitMQ publish + consume (trace via headers)
```

## Adapter packages

Optional subpackages of `boeng` instrument popular frameworks without
forcing your business code to import OpenTelemetry. Each opens a boeng
operation per call and records duration + error. HTTP and RabbitMQ also
propagate W3C trace context across process boundaries, so a request's
trace stays continuous from one service to the next.

| Adapter                       | Use it via                                           |
| ----------------------------- | ---------------------------------------------------- |
| `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/gin`        | `r.Use(boenggin.Middleware())`                       |
| `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http`       | `boenghttp.Middleware(handler)` / `boenghttp.Transport(...)` |
| `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo`      | `options.Client().SetMonitor(boengmongo.CommandMonitor())` |
| `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/redis`      | `client.AddHook(boengredis.Hook())`                  |
| `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/rabbit`     | `boengrabbit.Publish(ctx, ch, ex, key, msg)` / `boengrabbit.Consume(name, handler)` |

## Local observability stack

A full local stack — OTel Collector, Tempo, Prometheus, Loki, Jaeger,
Grafana — is wired in `compose/`:

```bash
cd compose
docker compose --profile mini up -d   # Loki + Grafana
docker compose --profile file up -d   # adds Promtail tailing ../logs/*.log
docker compose --profile full up -d   # adds Tempo, Jaeger, Prometheus,
                                      # OTel Collector, ClickHouse, Wiremock
```

Endpoints: Grafana `:3000`, Jaeger `:16686`, Prometheus `:9090`,
OTLP gRPC `:4317`.

## Package layout

```
boeng/             # the public Operation Runtime — import this
    gin/           # Gin middleware (optional adapter)
    http/          # net/http server + client (optional adapter)
    mongo/         # MongoDB CommandMonitor (optional adapter)
    redis/         # go-redis Hook (optional adapter)
    rabbit/        # RabbitMQ publish/consume with header propagation
internal/          # implementation; not part of the public API
    core/          # domain types, ports, services
    adapter/       # OTel + sink adapters
    logtrc/        # low-level fluent logger (formerly public; now internal)
    util/          # JSON writer, helpers
compose/           # local observability stack
examples/          # boeng usage demos
scripts/           # demo + smoke scripts
```

Anything under `internal/` is not part of the API contract and may
change between releases.

## For AI coding assistants

If you are an LLM editing a Go file that imports `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng`,
apply these rules verbatim:

**DO**

- Use `boeng.Run(ctx, "<op_name>", subject, fn)` to wrap a function body
  that returns an `error`. The op name is snake_case, the subject is
  the business struct (it doesn't need a Loggable method — reflection
  is the fallback).
- Use `op := boeng.Enter("...", subject); defer op.Close()` when you
  cannot wrap the body in a closure (legacy code, no ctx threaded).
- Use `op.Step("<phase>", func() error { ... })` to break an operation
  into named phases. Steps produce their own span, log line, and metric.
- Use `boeng.L(ctx).F("k", v).Info("...")` only for ad-hoc mid-flow
  log lines. Most logging is automatic via `Run`/`Enter`.
- Use `boenghttp.Transport(...)` for outgoing HTTP, `boenggin.Middleware()`
  for Gin servers, `boengmongo.CommandMonitor()`, `boengredis.Hook()`,
  `boengrabbit.Publish` / `boengrabbit.Consume` for those backends.
- Configure OpenTelemetry once. If the service already installs its own
  `TracerProvider` / `MeterProvider` / propagator, leave `Config.OTel`
  empty and boeng joins that pipeline. Set `Config.OTel` (a URL,
  `http://` or `https://`) only when boeng should be the exporter, and
  never copy `OTEL_EXPORTER_OTLP_ENDPOINT` into it as well.
- Mount `obs.MetricsHandler()` on the application's existing HTTP server
  when Prometheus must scrape; do not open a second listener for it.

**DO NOT**

- Do not import `log/slog`, `go.uber.org/zap`, `github.com/rs/zerolog`,
  or any other logging library inside business code. boeng IS the
  application-facing logger.
- Do not import `go.opentelemetry.io/otel/...` inside business code.
  Trace, span, propagator, meter — all of that lives behind boeng. If
  you find yourself wanting an OTel call, you want a boeng verb.
- Do not write `defer span.End()`, `defer cancel()`-style observability
  ladders, `metric.Add`, `counter.Inc`, or manual duration timers. They
  duplicate what `Run`/`Enter` already do.
- Do not put high-cardinality fields (`user_id`, `order_id`, `email`,
  `request_id`, free-form text, IPs) in `Config.MetricLabels`. They
  belong in logs and spans, never in metric labels.
- Do not call `boeng.SetSinkForTest` outside `*_test.go` files.

If existing code uses slog/zap/otel directly, prefer migrating it to
boeng over adding a parallel observability path.

## Status

Pre-v1.0. The `boeng` public surface is frozen — future work adds new
pipeline stages (audit, profiling, sampling tuning) without changing the
four verbs or the operation-handle methods. See [`CHANGELOG.md`](./CHANGELOG.md).

### Test pyramid

| Layer | Status |
| ----- | ------ |
| Unit | ✅ `boeng/{fields,enter,run,metrics,levels,l_spancontext}_test.go` |
| Metric surface | ✅ `boeng/{metrics_handler,metrics_labeled}_test.go` |
| OTel coexistence | ✅ `boeng/{otel_env,propagator,tracer_adopt,meter_adopt}_test.go` + `internal/adapter/outbound/otel/endpoint_test.go` |
| Dependency guard | ✅ `boeng/deps_guard_test.go` |
| Verification | ✅ `boeng/verify_test.go` |
| Runtime Guarantees | ✅ `boeng/guarantees_test.go` (G1–G7, see `boeng/README.md`) |
| API Freeze | ✅ `boeng/api_freeze_test.go` |
| Golden Trace | ✅ `boeng/golden_test.go` + `boeng/testdata/` |
| Adapter Compliance | ✅ `boeng/{gin,http,mongo,redis,rabbit}/*_test.go` |
| Integration (real backends) | ✅ `-tags integration` per adapter + `boeng/integration/chain_test.go` |
| Performance | ✅ `benchmarks/log_bench_test.go` |
| Performance & Memory Budget | ✅ `benchmarks/budget_test.go` — CI fails on regression |
| CI matrix | ✅ `.github/workflows/test.yml` |

Governance: [COMPATIBILITY.md](./COMPATIBILITY.md) defines the v1.x
semver contract. [ARCHITECTURE.md](./ARCHITECTURE.md) sketches the
mental model in one diagram.

> Verified against `6148b99` · 2026-09-18

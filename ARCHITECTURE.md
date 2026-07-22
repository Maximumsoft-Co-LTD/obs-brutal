# Architecture

**Everything starts from an Operation.**

```
            ┌────────────────────────────────────────────┐
            │            Business code                   │
            │   boeng.Run / Enter / EnterCtx / Emit      │
            └─────────────────────┬──────────────────────┘
                                  │ declares
                                  ▼
                  ┌───────────────────────────────┐
                  │          Operation            │
                  │  name + subject + outcome     │
                  └───────────────┬───────────────┘
                                  │ dispatched into
                                  ▼
                  ┌───────────────────────────────┐
                  │       Operation Runtime       │
                  └───────────────┬───────────────┘
                                  │
            ┌─────────────────────┴─────────────────────┐
            │                                           │
            ▼                                           ▼
   ┌─────────────────┐                       ┌─────────────────┐
   │     Pipeline    │                       │  Correlation    │
   │   (per stage)   │                       │  (per ctx)      │
   ├─────────────────┤                       ├─────────────────┤
   │  Logger         │                       │  trace_id       │
   │  Tracer         │                       │  span_id        │
   │  Meter          │                       │  parent linkage │
   │  Recover        │                       │  W3C propagator │
   │  Sampling       │                       └─────────────────┘
   │  Security/Mask  │
   │  Exporter       │
   │  Sink           │
   └────────┬────────┘
            │
            ▼
   ┌─────────────────┐
   │     Outputs     │
   ├─────────────────┤
   │  stdout JSON    │
   │  Loki           │
   │  OTLP           │
   │  alert webhooks │
   └─────────────────┘
```

That's the whole mental model.

## Why this shape

- **One concept, not twelve.** Business code knows about `Operation`.
  It does not need to understand spans, counters, histograms, log
  records, propagators, or exporters. Those exist; they live inside
  the runtime; the runtime decides how to fan one operation out across
  them.
- **The runtime owns the pipeline, not the call site.** Adding a new
  stage (audit, profiling, sampling, anomaly detection) is a runtime
  change, not an application change. The four verbs in business code
  stay the same.
- **Adapters are not features — they are *boundary translators*.**
  `boeng/http`, `boeng/rabbit`, `boeng/mongo`, `boeng/redis`,
  `boeng/gin` take an external signal (an HTTP request, an AMQP
  delivery, a Mongo command) and open exactly one Operation. Whatever
  the runtime does to that Operation is the same as for hand-written
  code. There is no second observability path.
- **Correlation is a context.Context concern, not an SDK concern.**
  Trace context propagates through `context.Context` for in-process
  hops and through W3C headers (HTTP) or AMQP table entries (RabbitMQ)
  across the wire. The application never touches the propagator.

## The four-verb invariant

If you can answer "what operation am I doing right now?" you can
instrument the code:

| Shape of code                          | Verb         |
| -------------------------------------- | ------------ |
| I want to wrap a function body         | `boeng.Run`  |
| I want to open an operation from inside a function | `boeng.Enter` / `EnterCtx` |
| I want to record a domain event        | `boeng.Emit` |
| I am setting up the process            | `boeng.Init` |

Everything else lives on `*Op`: `Step`, `Log`, `Emit`, `Fail`,
`Success`, `Close`, `CloseWith`, `Context`.

## What lives where

```
boeng/                             # public Operation Runtime
    gin / http / mongo / redis / rabbit   # boundary translators
internal/
    core/
        domain/                   # Level, LogEntry, Field, StructuredError
        port/                     # Sink, TelemetryProvider, ResponseBuilder
        service/                  # business logic of the pipeline stages
    adapter/
        outbound/sink/            # Stdout, File, Loki, Buffered, alerts, ...
        outbound/otel/            # OTel TracerProvider + MeterProvider
        outbound/telemetry/       # boeng-side metrics emission
    logtrc/                       # the low-level fluent logger (was public, now internal)
    util/                         # JSON writer, helpers
```

Everything inside `internal/` is the runtime's machinery. The contract
with users is the `boeng/` tree only — see
[`COMPATIBILITY.md`](./COMPATIBILITY.md).

## Pointers into the code

- Public surface: [`boeng/doc.go`](./boeng/doc.go)
- Runtime promises (tests, with names): [`boeng/guarantees_test.go`](./boeng/guarantees_test.go)
- API freeze: [`boeng/api_freeze_test.go`](./boeng/api_freeze_test.go)
- Golden trace: [`boeng/golden_test.go`](./boeng/golden_test.go) +
  [`boeng/testdata/golden_create_user.json`](./boeng/testdata/golden_create_user.json)
- AI transformation contract: [`boeng/transform_test.go`](./boeng/transform_test.go) +
  [`boeng/testdata/transform/`](./boeng/testdata/transform/)

> Verified against `0d0a832` · 2026-07-22

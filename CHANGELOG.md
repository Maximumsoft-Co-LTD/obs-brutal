# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `boeng` Operation Runtime package — single public entry point.
- Four top-level verbs: `Init`, `Run` (+ `RunR`), `Enter` / `EnterCtx`,
  `Emit`.
- Operation handle methods: `Step`, `Log`, `Emit`, `Fail`, `Success`,
  `Close`, `CloseWith`, `Context`.
- `Loggable` interface for domain types to control their log/span
  attribute shape from the domain layer.
- Reflection fallback when a subject doesn't implement `Loggable`:
  snake_case naming, zero-value skipping (toggleable), auto-masking of
  sensitive keys (`password`, `token`, `secret`, `email`, `phone`,
  `apikey`, `auth`, `ssn`, `credit_card`).
- Per-operation metrics: `<op>_total`, `<op>_duration_ms`,
  `<op>_error_total`, `<op>_panic_total`. Auto-emitted by `Run`,
  `Enter`, and `op.Step`.
- Cardinality-safe metric label allowlist via `Config.MetricLabels`.
  Subject fields are never silently promoted to labels.
- OTLP gRPC exporter is optional. If `Config.OTel` is empty or the
  collector is unreachable, boeng falls back to the configured sinks
  (stdout JSON by default) without panicking.
- Panic recovery is on by default for `Run` / `RunR` / `Enter` /
  `EnterCtx` / `op.Step`: the panic is recorded against the operation's
  span, counted as `_panic_total`, attached to the operation's error,
  and re-raised so the call stack unwinds normally.
- Apache 2.0 license.

### Changed (breaking from pre-release state)
- `logtrc` package moved to `internal/logtrc`. Users should import
  `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng` instead. The fluent `LogBrt` interface, the sink
  constructors (`NewFastStdoutSink`, `NewLokiPushSink`, etc.), and the
  logger constructors (`NewOTelWithService`, `NewAsyncLogBrt`, etc.)
  are no longer part of the public API surface.
- `boeng.Op` and `boeng.End` removed; replaced by `boeng.Run` /
  `EnterCtx` + `op.Close`.
- `boeng.Log(ctx, msg, subjects...)` removed from public API.
  Manual log lines live on the operation handle (`op.Log`).
- `op.Event` renamed to `op.Emit` so the verb matches the package-level
  `boeng.Emit`.
- `op.Log` signature swapped to `(msg string, subjects ...any)` so the
  message comes first, matching `Logger.Info` and friends.
- `obs:` struct-tag-driven extraction removed. Domain types control
  their log shape via the `Loggable` interface; types without it fall
  back to reflection.
- `Config.Masking` removed — it was a TODO field that never wired into
  anything.
- Pre-v1 demo programs (`basic`, `demo_all`, `promtail_file`,
  `otel_loki`, etc.) removed along with `scripts/demo.sh`. The examples
  that remain are `boeng`, `boeng_ctx`, `boeng_legacy` (the three
  documented usage modes) plus one small program per adapter
  (`cli`, `cron`, `gin`, `http`, `mongo`, `rabbit`, `redis`).

### Fixed
- Metric-name cardinality now fails closed (the second half of G3).
  `opMetricsFor` / `eventMetricsFor` cap the number of distinct
  sanitized metric names at `maxDistinctMetricNames` (512); past the
  cap, names route to a shared `overflow` series instead of minting a
  new one. Previously the caches were unbounded maps, so any op name
  built from high-cardinality data minted a new metric series per
  unique value — leaking the map and able to OOM the downstream
  Prometheus/Mimir. The overflow counter is the operator's signal.
- HTTP / Gin / RabbitMQ adapters no longer build high-cardinality op
  (hence metric) names from request data — which also protects the
  shared name budget above so a chatty adapter can't starve an app's
  business-op metrics:
  - `boenghttp.Middleware` names the op `HTTP <METHOD>` instead of
    `<METHOD> <raw-path>` (plain net/http exposes no route template);
    the full path stays in the `http.path` field. Use `Wrap` with an
    explicit name for per-route metrics.
  - `boenggin.Middleware`'s unmatched-route fallback is `<METHOD>
    [unmatched]` instead of the raw URL path (matched routes still use
    the bounded `c.FullPath()` template).
  - `boengrabbit.Publish` names the op `rabbit.publish <exchange>`
    instead of `<exchange>/<routing-key>`; the routing key (which
    routinely embeds ids) stays in the `messaging.rabbitmq.routing`
    field.
- `Close` now actually flushes the async pipeline: `AsyncPipeline.Stop`
  drains queued entries and lets sink workers finish in-flight batches
  before the goroutines exit. Previously Stop cancelled the workers
  outright, so a process shorter than the 100 ms flush interval (any
  CLI, cron job, or crashing service) emitted **no logs at all** in
  OTel/Async mode. Writes issued after Stop fall back to the
  synchronous path instead of vanishing into a dead queue.
- Fluent chaining (`F` / `Fs` / `Ctx`) on the OTel-mode logger derives
  a new logger instead of mutating the shared one. In-place mutation
  meant every field ever attached (`user_id`, `error`, `span_id`, …)
  leaked into all subsequent operations' log lines — unrelated ops
  reported each other's errors and correlation ids. `StrategyLogBrt`
  and `OTelLogBrt` now clone on chain, matching the underlying
  `UnifiedLogBrt` semantics.
- Per-operation metrics now reach the OTLP collector: the meter
  provider gained an `otlpmetricgrpc` periodic reader (10 s interval,
  flushed on `Close`) targeting the same `Config.OTel` endpoint as
  traces. Previously metrics only fed an in-process Prometheus
  registry that nothing scraped, so `<op>_total` / `_duration_ms` /
  `_error_total` / `_panic_total` never appeared in the documented
  compose stack.
- Loki sink outage behavior hardened (found by failure-mode testing):
  the retry buffer is now capped at 10× the batch size (oldest entries
  drop, counted) instead of growing unboundedly for the duration of an
  outage; a post-failure backoff gate stops every `Write` from
  attempting a synchronous connection while Loki is down; `Close` is
  idempotent instead of panicking on a double close; and the flush
  worker no longer deadlocks on a nil channel after `Close`.
- Performance budget tests skip themselves under `-race` (the budgets
  are sized for uninstrumented builds), so `go test -race ./...` now
  runs clean as a whole.
- Alert sinks (Slack / Telegram / Opsgenie) hardened (found by a
  parallel bug-hunt over the untested sink packages, each finding
  reproduced by a failing test before the fix):
  - Transport errors no longer leak the secret-bearing request URL
    (Telegram bot token, Slack webhook) into the error returned from
    `Write`; failures are reported by category only.
  - Permanent 4xx responses are no longer retried (a revoked webhook
    was hammered 5×/entry); only 5xx and 429 are retryable.
  - The retry loop is bounded by a total time budget and per-attempt
    context timeout, so a dead alert endpoint can no longer stall the
    synchronous log path for ~28s per entry.
  - `Configure` now locks, fixing a data race with `Write` on the
    webhook/token/endpoint fields.
- `async.Sink` (async wrapper) fixed: `Close` now drains queued entries
  instead of racing its workers into dropping them, `Write` after
  `Close` drops quietly instead of panicking on a closed channel,
  `Close` propagates to the wrapped inner sink, and `Close` is
  idempotent. The broken `WithTimeout` helper (auto-closed the sink
  permanently while claiming "periodic flush") was removed.
- `buffered.BufferedSink` fixed: `Close` is idempotent (double close no
  longer panics), and `flushLocked` no longer swallows inner-sink write
  errors while discarding the entries — failed entries are retained for
  the next flush (bounded, oldest-dropped) and the error is surfaced.
- File sinks fixed: `LumberjackSink.Write` captures the logger under the
  lock (no more nil-panic/race when `Configure` rebuilds it);
  `OptimalFileSink` locks `Configure`, honors a filename change after
  the first write, no longer truncates the live log when a rotation
  rename fails (append-only reopen), no longer wedges permanently after
  a failed rotation reopen, and both file sinks now implement `Close`
  (the file handle was previously leaked on shutdown).
- `Init` called twice now closes the previous default before replacing
  it, so the old async pipeline + OTel exporter no longer leak.
- `MaskEmail` operates on runes instead of bytes; multi-byte local
  parts (`jöhn@doe.com`, `พี่@example.com`) stay valid UTF-8.
- JSON writer now emits `user_id`, `module`, and `tenant_id` as
  top-level fields when promoted by the core (`Promote()`) — they were
  previously deleted from the entry without being re-emitted.
- `TestBudget_RunErrorPath` time budget raised from 8 µs to 12 µs
  (`budgetRunErrNsPerOp`), and the measurement now redirects stdout to
  `/dev/null`: the error path writes a JSON error log per op, so the
  benchmark-sized loop both flooded the CI log past its truncation
  limit and measured GitHub's log pipe instead of the library — the
  jobs failed on runner I/O speed rather than a real regression. The
  budget still covers encode + write; alloc and byte budgets are
  unchanged.

### Compose stack
- Added Grafana Mimir (`:9009`) to the `full` profile as a long-term
  metrics backend. The OTel Collector's metrics pipeline now exports to
  both its local Prometheus endpoint and Mimir via OTLP HTTP
  (`otlphttp/mimir`), and Grafana ships a provisioned `Mimir`
  datasource pointing at Mimir's Prometheus-compatible query API.
  Application code is unchanged — metrics already leave the process as
  OTLP to `Config.OTel`.

### Architectural
- Established the **Runtime + Pipeline** architecture: business code
  declares an operation; the runtime dispatches it through Logger →
  Trace → Metric → Recover → Sampling → Security → Exporter → Sink.
  Future pipeline stages (audit, profiling) add without changing the
  public API.

### Adapter packages (optional)

Five subpackages instrument popular frameworks without forcing
application code to import OpenTelemetry. Each opens a boeng operation
per call site and records duration + error automatically; HTTP and
RabbitMQ also propagate W3C trace context across process boundaries.

- `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/gin` — Gin middleware (`boenggin.Middleware`,
  `boenggin.L(c)`).
- `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http` — `boenghttp.Middleware` /
  `boenghttp.Wrap(name, handler)` for servers, `boenghttp.Transport(inner)`
  for clients. Outgoing requests carry a `traceparent` header; incoming
  requests extract it.
- `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo` — `boengmongo.CommandMonitor()` for use with
  `mongo.Connect(..., options.Client().SetMonitor(...))`.
- `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/redis` — `boengredis.Hook()` for use with
  `client.AddHook(...)`.
- `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/rabbit` — `boengrabbit.Publish(...)` injects
  `traceparent` into `amqp.Publishing.Headers`; `boengrabbit.Consume(name, handler)`
  extracts it so the consumer's op nests under the producer's trace.

`Logger` is now exported from `boeng` so adapter signatures don't leak
the internal `logtrc.LogBrt` type name.

`boeng.Init` now installs the W3C TraceContext + Baggage propagator
globally on every call, so trace propagation works even when the OTel
exporter is disabled.

### Test pyramid

The boeng v1.0 release defines a fixed test pyramid; every layer must
remain green before tagging a release.

| Layer | File | Promise |
| ----- | ---- | ------- |
| Unit | `boeng/{fields,enter,run,metrics}_test.go` | Each function behaves correctly. |
| Verification | `boeng/verify_test.go` | User-facing scenarios run end-to-end. |
| Runtime Guarantees | `boeng/guarantees_test.go` | The 7 README-published guarantees. |
| Golden Trace | `boeng/golden_test.go` + `testdata/golden_*.json` | Stable, redacted log shape of the canonical operation tree. |
| API Freeze | `boeng/api_freeze_test.go` | Removing a public symbol fails `go build`. |
| Adapter Compliance | `boeng/{gin,http,mongo,redis,rabbit}/*_test.go` | Each adapter opens a child op and propagates context across its medium. |
| Integration | `boeng/{mongo,redis,rabbit}/integration_test.go` (`-tags integration`) | Each adapter actually drives its real backend; skipped when unreachable. |
| End-to-end chain | `boeng/integration/chain_test.go` (`-tags integration`) | HTTP → Rabbit → Redis → Mongo produces a boeng op at every hop. |
| Performance | `benchmarks/log_bench_test.go` | `Run`, `RunWithSubject`, `RunErrorPath`, `EnterStep` benchmarks. |

CI matrix (`.github/workflows/test.yml`) runs all layers on every push:
unit + verification + guarantees + golden + freeze on a plain runner,
integration with MongoDB/Redis/RabbitMQ as GitHub Actions services,
plus a benchmark-compiles smoke gate.

Goldens regenerate via `go test -run TestGoldenTrace ./boeng/ -update`.
API freeze fails the build deliberately if a public symbol is removed
or renamed — extend `freezeRefs` when adding new public surface; remove
references only in a documented major-version bump.

> Verified against `0d0a832` · 2026-07-22

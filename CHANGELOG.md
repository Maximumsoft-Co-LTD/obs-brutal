# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Driven by the hash-central / slip-verify adoption report on v1.2.4.

### Added
- `Config.QuietOps` — demotes the success-path `<op> completed` line to
  DEBUG so production can run at `Level: InfoLevel` with events visible
  and no INFO line per operation. `<op> failed` stays ERROR.
- `Config.EmitLevel` — level of `Emit` / `op.Emit` event lines (zero =
  INFO). Set `WarnLevel` when a deployment runs at `WarnLevel` and still
  needs its per-request summary events.
- `(*Obs).MetricsHandler()` — serves the in-process Prometheus registry
  (`<op>_total`, `<op>_duration_ms_*`, `<op>_error_total`,
  `<op>_panic_total`, `<event>_total`) for pull-based scraping when the
  OTLP collector does not accept metrics. With `Config.OTel` empty the
  first call turns metric recording on.
- `boeng.L(ctx)` now recovers `trace_id` / `span_id` from any valid OTel
  span in `ctx` (otelhttp, otelgin, hand-rolled middleware), not only
  from spans boeng opened. A boeng operation in `ctx` still takes
  priority.
- `Config.MetricSchema` with `PerOpMetrics` (default, unchanged),
  `LabeledMetrics` and `BothMetrics`. `LabeledMetrics` emits one fixed
  family — `boeng_operation_duration_seconds{op,outcome}` (seconds,
  1 ms…10 s buckets, `_count` is the total) and
  `boeng_events_total{event}` — so `sum by (op)` works and no series is
  minted per operation name. `op` shares the 512-distinct-names cap.
- `boeng/README.md`: "Which lines you see at which level", "Reserved
  keys", "Names as Prometheus sees them", "Pull-based /metrics", and the
  `http://` scheme guidance for `Config.OTel` /
  `OTEL_EXPORTER_OTLP_ENDPOINT`.

### Changed
- boeng now joins an existing OpenTelemetry setup instead of replacing
  it. With no endpoint configured and an SDK `TracerProvider` already
  installed by the application, `Init` opens spans on that provider
  (resource, sampler, exporter all the application's) and leaves it
  running on `Close`. This takes precedence over an
  `OTEL_EXPORTER_OTLP_ENDPOINT` in the environment, which is assumed to
  belong to the application's pipeline; an explicit `Config.OTel` still
  wins. Previously `Init` always installed a private provider as the
  process global.
- boeng never replaces an application-installed SDK `MeterProvider`:
  its instruments record into it, and `MetricsHandler` answers 503 with
  an explanation instead of installing boeng's own provider over it.
- `Init` no longer overwrites a propagator the process already set. The
  `TraceContext + Baggage` composite is installed only when the global
  propagator is empty.
- `Run` / `Enter` without a prior `Init` open spans on the global
  tracer instead of returning a no-op span.
- The core `boeng` package no longer links Gin (and, through it, quic-go
  and the Mongo BSON library) into consumers. The unused Gin-bound
  helpers in the internal `logtrc` facade and the internal
  `adapter/inbound/log` package were removed; `boeng/gin` is the Gin
  integration. `TestCoreBoengLinksNoFrameworks` guards this.
- `Config.OTel` is now a URL per the OTel exporter spec and the scheme
  decides transport: `https://…` dials with TLS (system roots) — before
  1.3.0 the scheme was stripped and every endpoint was dialled in
  plaintext. `http://…` and bare `host:port` keep dialling plaintext.
- `Config.OTel` empty no longer means "never export". When
  `OTEL_EXPORTER_OTLP_ENDPOINT` (or `_TRACES_ENDPOINT` /
  `_METRICS_ENDPOINT`) is set in the environment, boeng exports and lets
  the SDK read endpoint, headers, timeout and certificates from the env,
  as every OTel SDK does. `Config.Service` is still required to export.
  **A service that set that variable while relying on an empty
  `Config.OTel` to stay silent will start exporting.** Unset the
  variable to keep the old behaviour.
- `<op>_duration_ms` histogram no longer declares OTel unit `ms`. The
  Prometheus exporter and collector translations were appending the
  unit, producing `<op>_duration_ms_milliseconds_bucket`; the series is
  now `<op>_duration_ms_bucket` / `_sum` / `_count` as documented.
  **Dashboards and alerts written against the `_milliseconds` name must
  be updated.**

## [1.2.3] - 2026-07-22

First tagged release to ship the `boeng` Operation Runtime. Everything
listed below first appeared in this tag. Note: replacing the public
`logtrc` surface with `boeng` is a breaking change relative to the
earlier tags, so under strict SemVer this would warrant a major bump —
it shipped as `v1.2.3` continuing the existing `v1.x` line.

Tags `v1.0.0`–`v1.2.2` (2025-08-18 … 2025-09-11) predate `boeng`. They
covered the standalone `logtrc` logging library that now lives under
`internal/logtrc`; those releases are not itemized here — see the git
history (`git log v1.0.0..v1.2.2`) for that era.

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
- Trace context now works without an OTLP exporter, fulfilling what
  `Init` documents ("propagation works even when no OTLP exporter is
  configured"). Previously a tracer was only installed when
  `Config.OTel` was set, so with no collector `boeng.Run` minted no
  span — meaning no `trace_id`/`span_id` in logs and no `traceparent`
  to propagate across process boundaries. `Init` now always installs a
  TracerProvider; only the OTLP *export* is gated on `Config.OTel`.
  Without an exporter the tracer uses `NeverSample`, so spans are
  non-recording (cheap) but still carry a valid W3C context — logs get
  correlation ids and adapters (HTTP, RabbitMQ) propagate `traceparent`
  with no collector. Per-op metric recording stays a no-op without a
  collector (the global meter provider is only taken over when
  exporting), so a no-OTel deployment keeps its prior metric cost.
  Budget baselines rose accordingly (Run 26→37 allocs, Run-error
  53→77) — the deterministic cost of attaching trace context to every
  op; the golden log shape is unaffected (trace ids are redacted as
  volatile).
- `Obs.Close` bounds OTel shutdown with a 5s deadline instead of
  `context.Background()`. `TracerProvider`/`MeterProvider.Shutdown`
  flushes through the OTLP exporter, which retries on a dead collector,
  so an unbounded context made process exit block for the exporter's
  full retry window (~1 min) whenever the collector was unreachable at
  shutdown. Verified: against a black-hole endpoint Close now returns
  in ~5s instead of hanging.
- `configureMetrics` (called by `Init`) now resets the per-op and
  per-event metric instrument caches. `Init` installs a fresh global
  MeterProvider, so without the reset a second `Init` (hot-reload,
  tests) left cached instruments bound to the previous, now shut-down
  provider — their metrics silently stopped being recorded.
- `RateSampler.ShouldSample` is now concurrency-safe. It ran on the
  OTel-mode logging hot path (every entry passes through the strategy
  Manager, which calls samplers under only a read lock, so many
  goroutines invoke it at once) yet consulted a private `*rand.Rand`,
  which is not safe for concurrent use — a data race under `-race` and
  undefined behaviour under load. Access is now mutex-guarded, and the
  rate=1 (always-on, the default) and rate=0 cases short-circuit the
  rng entirely. `AdaptiveSampler` already used the (safe) global rand.
- Metric-name cardinality now fails closed (the second half of G3).
  `opMetricsFor` / `eventMetricsFor` cap the number of distinct
  sanitized metric names at `maxDistinctMetricNames` (512); past the
  cap, names route to a shared `overflow` series instead of minting a
  new one. Previously the caches were unbounded maps, so any op name
  built from high-cardinality data minted a new metric series per
  unique value — leaking the map and able to OOM the downstream
  Prometheus/Mimir. The overflow counter is the operator's signal.
  The HTTP / Gin / RabbitMQ adapters keep their specific, per-endpoint
  op/span names (`GET /users/:id`, `rabbit.publish <exchange>/<key>`)
  so traces and logs stay granular — metric-name cardinality is bounded
  by the fail-closed cap above, not by flattening the names.
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
- Performance budget: the `ns/op` gate is now informational on CI
  (`CI=true`) rather than a hard failure. Wall-clock swings 3–10x on
  shared CI runners (no CPU pinning, noisy neighbours), so it failed on
  machine noise, not real regressions. The deterministic `allocs/op`
  and `bytes/op` gates — which catch genuine regressions — still fail
  the build everywhere; `ns/op` remains a hard gate on developer
  machines.
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

[Unreleased]: https://github.com/Maximumsoft-Co-LTD/obs-brutal/compare/v1.2.3...HEAD
[1.2.3]: https://github.com/Maximumsoft-Co-LTD/obs-brutal/releases/tag/v1.2.3

> Verified against `6148b99` · 2026-09-18

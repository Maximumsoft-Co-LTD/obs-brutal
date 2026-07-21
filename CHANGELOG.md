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
- All non-`boeng` example programs (`basic`, `gin`, `http`, `mongo`,
  `redis`, etc.) removed. The three remaining examples (`boeng`,
  `boeng_ctx`, `boeng_legacy`) cover the three documented usage modes.

### Fixed
- `Init` called twice now closes the previous default before replacing
  it, so the old async pipeline + OTel exporter no longer leak.
- `MaskEmail` operates on runes instead of bytes; multi-byte local
  parts (`jöhn@doe.com`, `พี่@example.com`) stay valid UTF-8.
- JSON writer now emits `user_id`, `module`, and `tenant_id` as
  top-level fields when promoted by the core (`Promote()`) — they were
  previously deleted from the entry without being re-emitted.

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

# Testing knowledge

This document indexes every test layer in this repo and names exactly
what each one proves. New contributors land here when a test fails;
existing contributors land here before adding a layer.

The principle: **every promise in the README is gated by an executable
check**. If a layer below goes red, a documented promise has broken.

## Layers

### 1. Unit tests
**Where:** `boeng/{fields,enter,run,metrics,levels,l_spancontext,metrics_handler,metrics_labeled}_test.go`, internal package `*_test.go`.
**Proves:** each individual function does what its godoc says, in isolation.
**Run:** `go test ./boeng/`

### 2. Verification tests
**Where:** `boeng/verify_test.go`.
**Proves:** end-to-end user promises — `TestRunSuccess`, `TestRunError`,
`TestRunPanic`, `TestEnterLegacy`, `TestEnterCtxChildSpan`,
`TestMetricCardinalityGuard`, `TestOTLPFallback`.
**Run:** `go test -run "TestRun|TestEnter|TestMetricCardinality|TestOTLPFallback" ./boeng/`

### 3. Runtime guarantees
**Where:** `boeng/guarantees_test.go`.
**Proves:** the seven published guarantees in
[`../boeng/README.md`](../boeng/README.md) (G1–G7) — every operation
produces a structured log line, reports duration, has cardinality-safe
metric labels, captures and re-raises panics, preserves correlation,
adapters accept ctx, and exporter failure degrades gracefully.
**Run:** `go test -run TestGuarantee ./boeng/`

### 4. API freeze
**Where:** `boeng/api_freeze_test.go`.
**Proves:** no publicly exported boeng symbol has disappeared or had its
signature changed without an explicit major-version bump. The check is
compile-time: removing any tracked identifier breaks `go build`.
**Run:** `go test -run TestPublicAPI ./boeng/`
**To extend:** when adding new public surface, append to `freezeRefs` in
the same file.

### 5. Golden trace
**Where:** `boeng/golden_test.go` + `boeng/testdata/golden_create_user.json`.
**Proves:** a canonical `Run → Step → Emit` operation tree produces the
exact, redacted log stream we committed to. Catches silent shape drift
(reorder, level change, dropped field).
**Run:** `go test -run TestGoldenTrace ./boeng/`
**To regenerate after an intentional change:** `go test -run TestGoldenTrace ./boeng/ -update`

### 6. Migration test
**Where:** `boeng/migration_test.go`.
**Proves:** adding boeng to a function does NOT change the function's Go
type (same signature, same outputs for same inputs) — only adds
observability output. `reflect.TypeOf` enforces type equality bit-for-bit.
**Run:** `go test -run TestMigration ./boeng/`

### 7. AI transformation test
**Where:** `boeng/transform_test.go` + `boeng/testdata/transform/{case}/before,after.txt`.
**Proves:** the canonical edit an AI agent must make to add boeng to a
function stays within a published LOC budget (3–5 lines per case), AND
the "after" file does not leak banned imports (`log/slog`, `zap`,
`zerolog`, `go.opentelemetry.io/otel`).
**Run:** `go test -run TestAITransformation ./boeng/`

### 8. Adapter compliance (in-process)
**Where:** `boeng/{gin,http,mongo,redis,rabbit}/*_test.go`,
`boeng/http/propagation_test.go`.
**Proves:** each adapter accepts a parent ctx, opens an op, and (for
HTTP / RabbitMQ) propagates W3C trace context across its medium. Uses
httptest / in-memory fixtures — no real backend required.
**Run:** `go test ./boeng/...`

### 9. Integration (real backends)
**Where:** `boeng/{mongo,redis,rabbit}/integration_test.go` + `boeng/integration/chain_test.go`
(build tag `integration`).
**Proves:** the adapter actually drives its real backend AND, in the
chain test, that one HTTP request fans out through Rabbit → Redis →
Mongo with a boeng op recorded at every hop. Skipped cleanly if the
backend isn't reachable; required to pass in CI where the services are
running.
**Run locally:** `docker compose --profile full -f compose/docker-compose.yml up -d`
then `go test -tags integration ./...`
**CI:** see [`../.github/workflows/test.yml`](../.github/workflows/test.yml)
job `integration`.

### 10. Performance benchmarks
**Where:** `benchmarks/log_bench_test.go`, `benchmarks/comparison_bench_test.go`.
**Reports:** ns/op + allocs/op + B/op for `Run`, `RunWithSubject`,
`RunErrorPath`, `EnterStep`, and the comparative bench against
`log.Printf` / `slog.LogAttrs` / `boeng.Emit`.
**Run:** `go test -bench=. -benchmem ./benchmarks/`
**Not a CI gate** — these are numbers, not pass/fail.

### 11. Performance & memory budget (CI gate)
**Where:** `benchmarks/budget_test.go`.
**Proves:** boeng paths stay within published budgets:
- `Run` ≤ 6 µs / 50 allocs / 4 KB per op
- `Emit` ≤ 3 µs / 30 allocs / 2.5 KB per op
- `Enter+Step` ≤ 12 µs / 100 allocs / 8 KB per op
- Memory per op stays flat from 1 to 1000 goroutines (no per-goroutine
  state leak).

Budgets are set ~3× observed M2 numbers to absorb CI runner variance.
A regression past any line fails CI.
**Run:** `go test -run TestBudget ./benchmarks/`

### 12. OTel coexistence
**Where:** `boeng/{otel_env,propagator,tracer_adopt,meter_adopt}_test.go`,
`internal/adapter/outbound/otel/endpoint_test.go`.
**Proves:** `Config.OTel` URL → transport (`https://` TLS, `http://` /
bare plaintext); export via `OTEL_EXPORTER_OTLP_ENDPOINT` when
`Config.OTel` is empty; an application-installed TracerProvider,
MeterProvider or propagator is adopted, never replaced or shut down
(spans land in the app's exporter, metrics in the app's reader,
`MetricsHandler` answers 503 instead of taking over); `Run` without
`Init` uses the global tracer. Export-mode tests talk to an in-process
gRPC server that answers `Unimplemented`, so `Close` returns at once.
**Run:** `go test -run "TestInit_|TestRun_WithoutInit|TestObs_|TestResolveEndpoint|TestExportConfigured" ./boeng/ ./internal/adapter/outbound/otel/`

### 13. Dependency guard
**Where:** `boeng/deps_guard_test.go`.
**Proves:** `go list -deps` of the core `boeng` package contains no Gin,
quic-go, Mongo, RabbitMQ or Redis package — adapters pay for their own
frameworks. Skipped under `-short`.
**Run:** `go test -run TestCoreBoengLinksNoFrameworks ./boeng/`

## CI matrix

[`../.github/workflows/test.yml`](../.github/workflows/test.yml) runs
three jobs per push:

| Job | Layers covered |
| --- | -------------- |
| `unit` | 1, 2, 3, 4, 5, 6, 7, 8, 11, 12, 13 (everything backend-free) |
| `integration` | 9 (with MongoDB / Redis / RabbitMQ as GitHub Actions services) |
| `benchmark` | 10 (smoke gate that benchmarks still compile + produce output) |

All three must be green for a PR to merge.

## Coverage

Current `go test -cover ./boeng/` reports **~86.5%** of statements in
the `boeng` package. That number is informational — the real measure
of completeness in this repo is **every Runtime Guarantee has a
named test** (currently G1–G7, all present) and **every public
symbol is in the freeze list** (currently 100%, enforced by layer 4).

## Open questions

None. Add new ones here when they appear.

> Verified against `6148b99` · 2026-09-18

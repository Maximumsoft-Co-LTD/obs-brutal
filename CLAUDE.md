# CLAUDE.md

Entry point for AI coding assistants working in this repository.
This file points at canonical documentation — it does not duplicate it.

## What this repository is

`obs-brutal` ships **one public Go package**: `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng`. It is
an **Operation Runtime** — application code declares business operations
(`boeng.Run` / `Enter` / `Emit`) and the runtime produces structured
logs, distributed traces, per-operation metrics, panic recovery, and
W3C trace propagation behind a single API. Everything under `internal/`
is implementation detail and not covered by the v1.x semver contract.

The full thesis lives in [`README.md`](./README.md). The mental model
lives in [`ARCHITECTURE.md`](./ARCHITECTURE.md). The semver policy
lives in [`COMPATIBILITY.md`](./COMPATIBILITY.md).

## Required reading order

Before editing any file in this repo, load these in order:

1. [`README.md`](./README.md) — product framing, four verbs, supported Go versions
2. [`ARCHITECTURE.md`](./ARCHITECTURE.md) — one-diagram mental model
3. [`boeng/README.md`](./boeng/README.md) — full API surface and runtime guarantees G1–G7
4. [`docs/walkthrough.md`](./docs/walkthrough.md) — what each call **actually emits** in trace / log / metric (verified by `boeng/walkthrough_test.go`)
5. [`COMPATIBILITY.md`](./COMPATIBILITY.md) — what counts as a breaking change
6. [`docs/ai/guardrails.md`](./docs/ai/guardrails.md) — explicit DO / DO NOT rules for code agents

## Context-loading rules (token-frugal)

- Start at [`docs/README.md`](./docs/README.md) — it is the navigation
  spine and links into every other doc.
- Do **not** read the whole `internal/` tree to understand boeng. The
  public surface is enumerated in `boeng/api_freeze_test.go` and the
  behaviour contract is in `boeng/guarantees_test.go`. Those two files
  are the source of truth for everything an application-facing change
  should respect.
- `internal/logtrc/` is the low-level fluent logger. It was public
  pre-v1; it is **internal** now. Anything you find that imports it
  from outside `github.com/Maximumsoft-Co-LTD/obs-brutal/internal/...` is a bug.

## Common commands

Run from module root (where `go.mod` lives).

```bash
# Build + vet + unit tests + verification + guarantees + golden + freeze
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...

# Single test
go test -run TestName ./boeng/...

# Performance + memory budget (gates CI; fails on regression)
go test -run TestBudget ./benchmarks/

# Benchmarks (numbers, not pass/fail)
go test -bench=Comparison -benchmem -benchtime=2s ./benchmarks/

# Integration (requires running mongo/redis/rabbit; otherwise skips)
go test -tags integration ./...

# Regenerate golden trace after an intentional schema change
go test -run TestGoldenTrace ./boeng/ -update
```

If `go test` errors with `package ... is not in std`, run
`go clean -cache -modcache` from the module root before retrying.
CGO is only needed on macOS for some tooling — see
[`README.md`](./README.md) for the "If you need CGO" guidance.

## Observability stack (`compose/`)

`compose/docker-compose.yml` defines a single stack with three Compose
profiles:

```bash
cd compose
docker compose --profile mini up -d   # Loki + Grafana (smoke-test Loki sinks)
docker compose --profile file up -d   # adds Promtail (tails ../logs/*.log)
docker compose --profile full up -d   # adds Tempo, Jaeger, Prometheus, OTel Collector, ClickHouse, Wiremock
```

Default endpoints: Grafana `:3000`, Prometheus `:9090`, Loki `:3100`,
Tempo `:3200`, Mimir `:9009` (OTLP ingest at `/otlp`, PromQL at
`/prometheus`), Jaeger UI `:16686`, OTLP gRPC `:4317`, Prometheus
scrape `:8889`, ClickHouse HTTP `:8123`, Wiremock `:8089`. Promtail
tails `../logs` mounted to `/var/log/app`.

`scripts/e2e_smoke.sh` runs the three boeng demo examples in the
background and dumps output to `logs/<name>.{out,err}`.

## High-risk areas

These are the code paths where a regression has the highest blast
radius. Touch them only with a corresponding test update:

- `boeng/run.go` — the four-verb dispatch + panic recovery contract.
  Changes here can break runtime guarantees G4 + G5.
  Tests: `boeng/run_test.go`, `boeng/guarantees_test.go`.
- `boeng/enter.go` — operation handle + `Step` + `CloseWith` panic
  re-raise. Subtle re-panic ordering; do not "simplify".
  Tests: `boeng/enter_test.go`, `boeng/guarantees_test.go`.
- `boeng/metrics.go` — per-op metric naming, the labeled schema, and
  the cardinality allowlist. Changes here can break G3 (cardinality
  fail-closed) and the SLO of every downstream Prometheus deployment.
  Tests: `boeng/metrics_test.go`, `boeng/metrics_labeled_test.go`,
  `boeng/metrics_handler_test.go`, `boeng/guarantees_test.go`.
- `boeng/boeng.go` `Init` + `internal/adapter/outbound/otel/provider.go`
  — where spans and metrics go (explicit endpoint → own provider;
  application provider → adopt; env → export; nothing → local), and
  the rule that boeng never replaces or shuts down an application's
  TracerProvider / MeterProvider / propagator.
  Tests: `boeng/otel_env_test.go`, `boeng/tracer_adopt_test.go`,
  `boeng/meter_adopt_test.go`, `boeng/propagator_test.go`.
- `boeng/fields.go` — Loggable + reflection fallback + sensitive-key
  auto-masking. UTF-8-safe; rune-counted.
  Tests: `boeng/fields_test.go` (includes multibyte regression cases).
- `internal/core/service/base/async.go` — embedded UnifiedLogBrt
  sinks are passed through; do **not** revert to default stdout. This
  was a real bug found in this checkout — see CHANGELOG.

## Knowledge maintenance rules

This repo carries a Convention Profile (see
[`docs/README.md`](./docs/README.md)) recording how docs do freshness,
anchors, index, cross-refs, protected content, and naming. Before
editing or adding any doc:

- **Freshness:** managed docs end with `> Verified against <sha> · <date>`.
  Refresh that stamp when you re-verify the doc against current code.
- **Anchors:** cite Go symbols (`boeng.Run`, `port.Sink`), not line numbers.
- **Cross-refs:** use relative markdown links of the form `[name](path)` — no wikilinks.
- **Protected:** the `<!-- HUMAN_AUTHORED -->` section at the bottom of
  this file is operational knowledge written by humans. Do not
  rewrite it.
- **API freeze:** never remove a symbol named in
  `boeng/api_freeze_test.go` without bumping major version per
  [`COMPATIBILITY.md`](./COMPATIBILITY.md).

## Review & testing rules

- Every code change must keep `go vet ./...` clean and
  `go test ./...` passing.
- Any change touching `boeng/*` must keep `TestBudget_*` green within
  the documented budget — see `benchmarks/budget_test.go`.
- Any change to the JSON output shape must regenerate the golden via
  `go test -run TestGoldenTrace ./boeng/ -update` AND be documented in
  [`CHANGELOG.md`](./CHANGELOG.md) under the next release.
- Any change to the public surface must add a corresponding line to
  `boeng/api_freeze_test.go` (for additions) or trigger a major version
  bump (for removals/renames).

## Open questions

None at the moment. New ones land in `docs/README.md` under "Open
questions" as they appear.

<!-- HUMAN_AUTHORED_START -->
<!--
  Operational notes preserved by humans. AI: do not rewrite this block.
  Add new notes by appending, never by replacing.
-->

- macOS + CGO note: some tooling (race detector, certain test runs)
  needs CGO. If `go test` errors with `package ... is not in std`,
  run `go clean -cache -modcache` from the module root, then retry.
- The compose stack endpoints above are the convention for local
  development; production endpoints come from environment, never
  from this file.

<!-- HUMAN_AUTHORED_END -->

> Verified against `6148b99` · 2026-09-18

<!-- claude-foundation:rules-imports:start (managed block — re-synced by install.sh; edit rules in .claude/rules/, not here) -->
## Always-on fundamentals

The `/dev` workflow's "by default" rules live in `.claude/rules/`. Recent Claude Code auto-loads that directory as project memory; the explicit import below is a fallback so the fundamentals still load on versions that do NOT auto-load `.claude/rules/`. If your Claude Code already auto-loads them, this import is redundant but harmless — delete this section if you ever see the router loaded twice.

@.claude/rules/fundamentals.md
<!-- claude-foundation:rules-imports:end -->

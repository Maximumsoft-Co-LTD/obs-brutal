# obs-brutal documentation index

This file is the navigation spine for everything in this repository.
Start here, load only the leaf doc you need.

## Reading paths

| If you are…                              | Start with                                                |
| ---------------------------------------- | --------------------------------------------------------- |
| A new user adopting boeng                | [`../README.md`](../README.md) → [`../boeng/README.md`](../boeng/README.md) |
| Asking "what will I actually see in logs / traces / metrics?" | [`walkthrough.md`](./walkthrough.md) — 8 scenarios with real captured output |
| An AI coding agent editing this repo     | [`../CLAUDE.md`](../CLAUDE.md) → [`ai/guardrails.md`](./ai/guardrails.md) |
| Curious about the mental model           | [`../ARCHITECTURE.md`](../ARCHITECTURE.md)                |
| Worried about backwards compatibility    | [`../COMPATIBILITY.md`](../COMPATIBILITY.md)              |
| Tracking what changed between releases   | [`../CHANGELOG.md`](../CHANGELOG.md)                      |
| Investigating a failing test             | [`testing.md`](./testing.md)                              |
| Tuning observability / SLOs              | [`../boeng/README.md`](../boeng/README.md) Metrics + Cardinality sections |

## Canonical documents

| Document                                       | Purpose                                                  |
| ---------------------------------------------- | -------------------------------------------------------- |
| [`../README.md`](../README.md)                 | Product framing, four verbs, supported Go, examples list |
| [`../boeng/README.md`](../boeng/README.md)     | Full API surface, Loggable contract, runtime guarantees G1–G7, test pyramid |
| [`../boeng/doc.go`](../boeng/doc.go)           | Package godoc — read by `go doc obs-brutal/boeng`        |
| [`../ARCHITECTURE.md`](../ARCHITECTURE.md)     | One-diagram mental model: Business → Operation → Pipeline → Outputs |
| [`../COMPATIBILITY.md`](../COMPATIBILITY.md)   | v1.x semver policy: what counts as breaking, deprecation rules |
| [`../CHANGELOG.md`](../CHANGELOG.md)           | Release history, breaking changes, test pyramid additions |
| [`../CLAUDE.md`](../CLAUDE.md)                 | AI-assistant entry point — required reading order        |
| [`walkthrough.md`](./walkthrough.md)           | 8 scenarios that answer "If I write this code, what exactly will I see in Trace, Log, and Metrics?" — every JSON line is verified against runtime by `boeng/walkthrough_test.go` |
| [`testing.md`](./testing.md)                   | All test layers, what each proves, how to run them       |
| [`ai/guardrails.md`](./ai/guardrails.md)       | DO / DO NOT rules for AI agents producing Go code        |

## What is NOT documented separately (and why)

This is a library, not a service. The following categories common to
service-shaped repos are intentionally collapsed into the canonical
documents above:

- **Workflows.** A library has no application-level workflows. The four
  verbs (`Run` / `Enter` / `Emit` / `Init`) plus the adapter table in
  [`../boeng/README.md`](../boeng/README.md) play that role.
- **Business rules.** The Runtime Guarantees G1–G7 are this library's
  business rules. They live in
  [`../boeng/README.md`](../boeng/README.md) and are pinned by
  `boeng/guarantees_test.go`.
- **Specifications.** `boeng/api_freeze_test.go` is the executable spec
  for the public surface; `boeng/guarantees_test.go` is the executable
  spec for runtime behaviour. A prose `specs/` directory would
  duplicate both without ever disagreeing — the doc-standards rule is
  that specs exist to make intent-vs-code drift visible, and there's
  no useful drift to expose here.
- **Incidents.** No production incidents in this codebase yet. The
  first one will create `.workflow/<run-id>/` per the existing repo
  convention.

## Performance

Reproducible benchmarks live in `benchmarks/`:

```bash
go test -bench=Comparison -benchmem -benchtime=2s ./benchmarks/
```

Categories:

- `BenchmarkComparison_*` — vs `log.Printf`, `slog.LogAttrs`, `boeng.Emit`,
  `boeng.Run`. The honest per-op cost of each layer.
- `BenchmarkRun*` / `BenchmarkEnterStep` — boeng-only paths.
- `TestBudget_*` — CI gates: fails if Run > 6 µs / 50 allocs / 4 KB, etc.
  (see `benchmarks/budget_test.go`).
- `TestBudget_MemoryUnderConcurrency` — proves per-op memory stays flat
  from 1 to 1000 goroutines.

A previous `cmd/perf_runner` benchmark suite existed pre-boeng. Its
results file (`perf_results.md`) was removed when the runner stopped
shipping; today's reproducible numbers supersede it.

## Open questions

None at the moment. Add new ones here when they appear so future
sessions don't re-discover them silently.

> Verified against `d6e1035` · 2026-06-28

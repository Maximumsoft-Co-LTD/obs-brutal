# Context: <run title>

> **For humans**: a one-time map of the *existing* code this run touches — read once, reused by every Phase-1 plan slice so none re-walks the codebase.

**Spec**: [./spec.md](./spec.md)
**Field**: brownfield · **Size**: M | L
**Built by**: `/spec` (or `/dev` orchestrator) after the spec check — brownfield M/L only.
**Consumed by**: `lead` (`/dev-plan`) → `## Current state` · `qa` (`/test-plan`) → `## Test infra` + invariants · `uxui` (`/uxui-plan`) → `## UI surface` · `engineer` (Phase-2 implement) → `## Current state` (orientation only — locate + flow, not a re-walk; loaded when `context_built`). Each reads this as the shared baseline and LSP-verifies only its own deltas; greenfield / XS-S runs skip this file entirely (slices cold-walk as before).
**Evidence, not authority**: a wrong fact here hits all three slices, so every consumer **spot-checks load-bearing claims** (re-resolve a sample of `path#anchor`s) and **owns its final map** (`plan.md` Current state / `test-plan.md` Baseline / `uxui-plan.md` UI surface) — never a blind copy of this file.

Procedure: `plan-writing > references/current-state.md` (boundary-depth, not a file tour — blast radius + invariants + insertion points, each `path#anchor`).

## Current state

**Entry point(s)**:
- `<path#anchor>` — <one-line role>

**Data / control flow** (LSP-walked, 3–7 hops):
1. `<path#anchor>` — <what this hop does> → calls `<symbol>` at `<path#anchor>`
N. `<path#anchor>` — terminal write / return / external call

**Callers / blast radius**:
- `<symbol>` (`<path#anchor>`): N callers — <summary; list non-obvious ones with path#anchor>
- `<symbol2>`: 0 callers — safe to change

**Invariants the current code relies on**:
- `<one-line invariant>` — `<path#anchor>` <why it's load-bearing>

## UI surface

<Delete if the run has no rendered surface. For `uxui`: what to reuse before inventing.>

**Design system / tokens**: `<path#anchor>` — <palette/type/spacing source>
**Components / routes**: `<Component or route>` (`<path#anchor>`) — <role, reuse vs extend>
**Styling approach**: <CSS modules / Tailwind / styled — `path#anchor`>

## Test infra

<Delete for `docs`/`chore`. For `qa`: what the Baseline / Regression contract builds on.>

**Runner / framework**: <name + config `path#anchor`>
**Fixtures / test data**: `<path#anchor>` — <seed, real-vs-doubled boundaries>
**Coverage of touched behaviour**: <covered by `path#anchor` | uncovered → characterization baseline needed before change>

## Discovered (ledger)

> Run-lifetime read-cache — the orchestrator (**single writer**) folds workers' `CONTEXT:` return lines here at every worker return; every subsequent spawn brief points here first. Created lazily at ANY size/field on the first returned fact (the sections above stay brownfield-M/L-only). **The diff wins** — entries touching files Implement changed are stale; mark or prune on fold. One line per fact, load-bearing only (invariant · entry point · gotcha); prune superseded lines.

- `path#anchor` — <fact> — [<phase>]

---
*Optional — keep only sections the run needs; `## UI surface` and `## Test infra` are deleted when not applicable. This is a shared **read input**, never a per-slice shard.*

# Run: <title>

> **XS micro-lane** — the single artifact replacing spec.md / plan.md / tasks.md / test-plan.md at `size=XS`. Same contract core (Goal · Type · `AC#` · `T###`+`verify:` · Coverage); `SIZE_UPGRADE: S` re-emits the full four-artifact set. S and above never use this file.

**ID**: `NNNN-type-slug`
**Type**: feat | fix | refactor | chore | docs | spike
**Size**: XS
**Field**: greenfield | brownfield
**Status**: draft | approved | done
**Open PR on ship**: yes | no

## Goal *(required)*

<one sentence: what changes, for whom / why now>

## Acceptance *(required — the contract; per-line confirmed at the gate)*

- [ ] **AC1** — **Given** <state>, **When** <action>, **Then** <outcome>.
- [ ] **AC2** — <boundary / on-error> — or `none — <default>`.

## Approach *(required — 1–3 lines: what changes where; fix: repro + expected; brownfield: the invariant not to break, `path#anchor`)*

<...>

## Tasks *(required)*

- [ ] **T001** [AC1] <action> — `path#anchor` (edit) — verify: <command or observable>

## Coverage *(feat/fix/refactor — one line per AC; chore/docs: `n/a — type=<x>`)*

- AC1 → <test / observable check> · Impacted cmd: `<command>`

---
*Assumptions (inferred) · deviations · notes — append below when real, delete otherwise.*

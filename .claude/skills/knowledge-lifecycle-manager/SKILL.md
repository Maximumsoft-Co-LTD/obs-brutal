---
name: knowledge-lifecycle-manager
description: >
  Use when onboarding a new or legacy repository, when documentation is missing,
  incomplete, outdated, or suspected to have drifted from source code. Builds and
  maintains an evidence-backed knowledge base including index, workflows, business
  rules, testing knowledge, specifications, incidents, AI guardrails, and CLAUDE.md.
  Unlike one-shot documentation generation, this skill manages the full documentation
  lifecycle through discovery, drift detection, incremental updates, verification,
  coverage tracking, and knowledge maintenance. Typical triggers include "generate
  project docs", "build knowledge base", "onboard repository", "analyze repository
  structure", "documentation drift", "update docs", "prepare for AI coding". Does not
  modify production code.
---

# Knowledge Lifecycle Manager

Build, maintain, verify, and evolve repository knowledge. The objective is not documentation
generation — it is **knowledge governance**: a navigable, drift-checked, evidence-backed picture of
the system that AI sessions and developers can trust without re-reading the whole codebase.

**Never modifies production code, tests, or business logic.** It reads source to derive docs and writes
docs. A bug or stale code found during discovery becomes an Open Question or incident candidate — not a fix.

## Core principles

1. **DO NOT GUESS** — no fabricated behavior.
2. **UNKNOWN > INCORRECT** — an explicit gap beats a confident error.
3. **DISCOVER, DON'T IMPOSE** — adopt the repo's conventions before inventing new ones.
4. **EVIDENCE REQUIRED** — every claim cites a source.
5. **RESPECT EXISTING CONVENTIONS** — extend what exists.
6. **INDEX FIRST** — the index is the navigation spine.
7. **VERIFY EVERYTHING** — completion is gated on executable checks, not vibes.
8. **MINIMIZE TOKENS** — link, don't duplicate.
9. **HUMAN KNOWLEDGE IS AUTHORITATIVE** — never overwrite human-authored content.

## Execution flow

**Assume a knowledge base may already exist.** ~90% of real runs are MAINTENANCE (audit + update an existing
KB), not bootstrap — and the dangerous failure mode is clobbering a current, human-maintained KB. Detect what's
there before generating anything; Discovery decides the mode via the Convention Profile.

Run in order. Each phase is a resumable checkpoint — update progress after each.

```
Repository Discovery → Convention Profile → Drift Analysis → Plan Gate
  → Update (maintenance)  /  Generate Missing (bootstrap)
  → Checkpoint → Verification → Reports
```

Drift must precede the Plan Gate (the NEW/UPDATE/SKIP plan *is* drift output). Verification runs before you
declare completion, and **compiles against the Convention Profile, not the skill's defaults**.

### 1. Repository Discovery → Convention Profile
Inspect `CLAUDE.md`, `README.md`, `docs/`, `.workflow/`, `.github/`, existing docs. **Output:** a Repository
Convention Report AND a **Convention Profile** (`references/convention-profile.md`) — a structured record of how
the repo ALREADY does freshness, anchors, index, incidents, cross-refs, protected content, and doc naming. The
profile drives generation, verification, and coverage downstream, and selects the **mode**: mostly populated →
MAINTENANCE; mostly `none` → BOOTSTRAP. Never replace a convention without evidence — extend what's there.

### 2. Drift Analysis
Compare documentation ↔ source. Use each managed doc's `source_commit` front matter to diff incrementally
rather than re-deriving everything. Classify each artifact `NEW` / `CHANGED` / `STALE` / `MISSING` /
`UNKNOWN`. **Output:** Drift Report. Prefer incremental updates.

### 3. Plan Gate
Classify every artifact `NEW` / `UPDATE` / `SKIP`, present the plan, get one approval point before writing.

### 4. Knowledge Generation / Update
Generate or update using `templates/`. Write rules (front matter, protected regions, index-summaries-only,
spec sourcing, CLAUDE.md, scope control) live in **`references/document-standards.md`** — load it before writing.

| Area | Location |
|------|----------|
| Index | `docs/index/` (README + system/workflow/business-rule/spec/test/incident maps) |
| Foundation | `docs/system-overview.md`, `repository-structure.md`, `dependency-map.md`, `risk-map.md` |
| Workflows | `docs/workflows/` |
| Business rules | `docs/business-rules/` |
| Testing knowledge | `docs/testing/` |
| Specifications | `specs/` |
| Incidents | the repo's **existing** incident convention (discover first — do not invent) |
| AI guardrails | `docs/ai/` |
| Entry point | `CLAUDE.md` |

Apply the **Confidence Gate** per artifact (`references/confidence-gate.md`) and enter **Grill-Me** for
unconfirmable items (`references/grill-me.md`). Honor **scope control**: document the Top-N highest-risk /
highest-traffic / business-critical flows first; mark the rest `MISSING`.

### 5. Checkpoint
After each major phase, update progress (e.g. `docs/index/progress.md`): phase, status, coverage, open
questions. The run must survive an interruption mid-phase.

### 6. Verification
Run the executable checks in **`references/verification-checks.md`** (CHECK-001 … CHECK-010), **compiling each
against the Convention Profile** — verify the convention the repo actually uses, not the skill's defaults. A
check that goes N/A merely because the repo's format differs is a *compile failure of the checklist*, not a
pass. **Output:** PASS / FAIL / N/A per check with evidence. The KB is not complete until all PASS or legitimately N/A.

### 7. Reports
Emit the Completion Report (below).

## Evidence (load `references/evidence-model.md`)

Every workflow, rule, spec, and testing artifact carries evidence. Primary evidence depends on mode:
**Bootstrap** (no KB yet) → code + tests + docs are ground truth. **Maintenance** (KB exists) →
workflow/rule/test/incident chain is primary; code is a *validation* oracle. Anchor on code symbols, not
line numbers.

## Coverage

Report **system coverage** (`documented / actual-in-code`), not file counts — see `references/coverage-recipes.md`
for the formulas. "10/10 docs current" is not 100% coverage; the denominator comes from source (routes, consumers,
cron jobs, env vars), and an underivable denominator is reported as UNKNOWN, never as a clean percentage.

## Context loading (for downstream AI sessions)

The product of this skill is built so future sessions start at `docs/index/README.md` and load only the
relevant leaf docs — never the whole set. That index-first design is the token payoff; preserve it.

## Reference loading (for this skill — load on demand, not all upfront)

- Discovery output → `references/convention-profile.md` (load FIRST — it drives generation, verification, coverage)
- Writing any doc → `references/document-standards.md` (freshness mapping, protected regions, index/spec/CLAUDE.md rules, scope)
- Evidence questions → `references/evidence-model.md`
- Confidence decisions → `references/confidence-gate.md`
- Human questions / Grill-Me → `references/grill-me.md`
- Verification → `references/verification-checks.md` (compile against the profile)
- Coverage metrics → `references/coverage-recipes.md` (system coverage, not file counts)
- Per-artifact structure → `templates/`; worked examples → `examples/`

## Completion criteria

Produce: Convention Report · Drift Report · Plan Report · Verification Report · Coverage Report ·
Generated Files · Updated Files · Missing Documents · Missing Tests · UNKNOWN sections ·
REQUIRES_HUMAN_REVIEW sections · Open Questions · Confidence Summary.

**Not complete** when: verification fails · index is outdated · managed regions are inconsistent ·
required reports are missing · open questions a phase needed to proceed remain unresolved.

## Skill relationship

Supersedes `init-project-docs` for repository knowledge generation/maintenance. Note: "redirect here" is
not self-enforcing — update `init-project-docs`'s own description to point here (or remove it), or both
may match the same trigger.

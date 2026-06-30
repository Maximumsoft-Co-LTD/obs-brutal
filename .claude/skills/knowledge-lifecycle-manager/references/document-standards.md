# Document Standards

The mechanical rules for *how* every managed document is written. SKILL.md points here; load when
you are about to create or update a file.

## Freshness & traceability (map onto the Convention Profile)

Every managed doc must declare the commit it was verified against — but **in the repo's existing form**, per
`profile.freshness` (see `convention-profile.md`):

- `stamp` → add/advance the repo's stamp line (e.g. `🕒 verified against <sha> · <date>`). Do **not** also add YAML — never two conventions for one concern.
- `frontmatter` → use the repo's existing YAML keys.
- `none` (bootstrap only) → introduce the skill-default YAML block below.

The *fields* matter, not the syntax — map `source_commit` / `status` / `confidence` onto whatever the repo
already uses. (In the hash-central run, advancing the `verified against <sha>` stamp WAS the front-matter update.)

Skill-default block (bootstrap, when `profile.freshness == none`):

```yaml
---
source_commit: <sha the doc was generated/verified against>
generated_at: <date>
generator: knowledge-lifecycle-manager
confidence: HIGH | MEDIUM | LOW
status: CURRENT | STALE | REVIEW | DEPRECATED
---
```

Why it exists — this is what turns *generation* into a *lifecycle*:
- **`source_commit`** lets Drift Analysis diff a doc against the exact code it was derived from, instead
  of re-deriving everything. Cheap, incremental, token-saving.
- **`status`** drives drift triage: `CURRENT` (matches source), `STALE` (source moved), `REVIEW`
  (evidence conflict / LOW confidence), `DEPRECATED` (superseded, kept for history).
- **`confidence`** records how solid the evidence was at write time (see `evidence-model.md`).

Verification asserts presence and validity: CHECK-007 (keys present), CHECK-009 (valid values),
CHECK-010 (`source_commit` resolves to a real commit).

## Protected regions

Human-authored content is authoritative and must never be overwritten. AI edits **managed** regions only.

```
<!-- HUMAN_AUTHORED_START -->
...content written by humans — this skill never rewrites this...
<!-- HUMAN_AUTHORED_END -->

<!-- MANAGED_START -->
...content this skill may regenerate...
<!-- MANAGED_END -->
```

Rules:
- When markers exist, only the `MANAGED` spans are editable.
- When a pre-existing doc has **no** markers, treat its prose as human-authored: append, don't overwrite,
  unless the Plan Gate classified it `UPDATE` with explicit user approval.
- Never delete or unbalance a `HUMAN_AUTHORED` marker pair (CHECK-008 fails the run if you do).
- This repository already relies on this: e.g. `CLAUDE.md` carries a "Do NOT collapse … reflects
  hard-won operational knowledge" note. Treat such notes as hard human-authored boundaries.

## Index documents (`docs/index/`)

The index is the navigation layer, context router, and token optimizer. Each entry stores **summaries
only** — title, one-line summary, references, risk, status. Never duplicate the leaf document's content
into the index; link to it. Every new leaf doc must be added to the relevant `*-map.md` in the same pass
(CHECK-001/005 fail otherwise).

## Specifications (`specs/`)

Specs are sourced from **workflows + business rules + testing knowledge — not generated directly from
source code.** Rationale (write this rationale into the spec index so future readers don't "fix" it):

- A spec captures *intended* behavior; code captures *actual* behavior. Keeping them separable is what
  makes drift between intent and implementation **detectable**.
- If specs were generated from code, a spec could never disagree with code, and drift would be invisible.

Code is allowed as a **validation oracle** (cross-check a spec assertion against a symbol to confirm it
holds) but never as the generation source. Any spec assertion with no backing workflow or test →
`REQUIRES_HUMAN_REVIEW`.

## CLAUDE.md

CLAUDE.md is an **entry point**, not a document dump. It references documentation; it does not duplicate
it. Edit only managed regions. Keep it to: project context, documentation structure, required reading
order, context-loading rules, high-risk areas, review/testing/human-review rules, knowledge-maintenance
rules, open questions.

## Scope control (what to document first)

Do not attempt the whole repository at once. Priority order:

1. Entry points → 2. APIs → 3. Message consumers → 4. Cron jobs → 5. High-risk areas →
6. High-traffic paths → 7. Business-critical flows.

Document the Top-N; mark the remainder `MISSING` in the index. Always record what was deferred — silent
truncation reads as "covered everything" when it isn't.

## Reference style

- Anchor on **code symbols** (`pkg.Func`), not line numbers (see `evidence-model.md`).
- Link between docs with the repo's existing cross-reference style; don't invent a new one.
- Link, don't duplicate — a fact lives in exactly one leaf doc; everything else references it.

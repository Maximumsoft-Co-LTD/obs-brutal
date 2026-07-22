# Review: <title>

**Plan**: [./plan.md](./plan.md) · **Tasks**: [./tasks.md](./tasks.md)
**Spec**: [./spec.md](./spec.md)
**Reviewed**: YYYY-MM-DD
**Verdict**: pass | fix-required
**Cycle**: 1 of max 2

## Tasks adherence *(required)*

One row per task (`tasks.md`) — no skipping rows. A deviation needs a one-line reason.

- [x] T001 — implemented as planned
- [ ] T002 — deviation: <what + why>

## Acceptance-criteria check *(required)*

One row per spec.md acceptance scenario (`AC#`), including each boundary/error scenario and any `measured:` target. Re-verify against the diff + running code (don't trust the checkbox). Any scenario that can't be ticked = **blocking**.

- [ ] AC1 — evidence: `path:line` / behaviour observed
- [ ] AC1 (boundary / error scenario) — evidence: `path:line` / behaviour observed

## Non-AC slot check *(required when spec has a Definition of Done or Constraints)*

- [ ] DoD: <item> — concrete artifact present? evidence: `path:line` (missing = **blocking**)
- [ ] Constraint: <constraint> — diff honours it? evidence: `path:line` (violation = **blocking**)

## Findings *(required)*

### Blocking

- `path:line` — issue → suggested fix

### Non-blocking

- `path:line` — note (carried to retro)

## Sign-off *(required)*

pass | fix-required → see Test

---

**Fanout-only sections** — add when that fanout ran:

- **Per-agent findings** — ≥ 2 review workers on one repo's diff; each `### team-<role>` with `**Dispatched-as**:` as its first line
- **Per-repo review** — one `### Repo: <path>` per changed repo; AC check / Verdict / Cycle stay global

Shape → **lead.md > Mode B (Review)** · per-repo → **orchestrator/references/fanout-dispatch.md > Lead — Mode B**.

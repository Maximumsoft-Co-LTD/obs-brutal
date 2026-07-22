# Retro: <title>

**Plan**: [./plan.md](./plan.md)
**Type**: feat | fix | refactor | chore | docs | spike
**Completed**: YYYY-MM-DD
**Total cycles**: review=N, test=N
**Run metrics**: wall-clock=<created_at → done_at (duration)> · size=<XS|S|M|L> · skipped=<n steps> · security=<fired|not-fired> · fanout=<phases fired vs single-pass, from state.json fanout_log>
**Ship**: commit=<sha> | PR=<url> | skipped (reason)

## What worked *(required)*

Specific + repeatable. "LSP-first nav saved a grep round on the auth middleware" beats "good process".

- ...

## What to change next time *(required)*

Each item paired with WHY. Vague entries get cut.

- ...

## Acceptance criteria status *(required)*

Final state of every acceptance scenario (`AC#`) in `spec.md > User Stories`.

- [x] AC1 — shipped
- [ ] AC2 — deferred → see Follow-ups

## SC outcome *(required — feat/fix/refactor; others `n/a — type=<x>`)*

Final state of every `SC-###` in `spec.md > Success Criteria` — measured, or explicitly unmeasurable (never silently dropped).

- SC-1 — met: <measured value / evidence>
- SC-2 — unmeasurable at ship → follow-up `F-<run-id>-NN` appended

## Memory candidates (facts) *(required)*

A single rule / preference / reference / user trait. Surface to the user for confirmation; never auto-save. Write "none this run" if empty. (Routing rules + save-worthy filters live in the retro agent.)

- **type**: feedback | project | reference | user
  **body**: <fact>
  **why**: <reason>
  **how to apply**: <when it kicks in>

  *(why + how required for feedback / project)*

## Skill candidates (procedures) *(required)*

A multi-step procedure — ≥ 3 steps OR conditional logic, AND a clear trigger, AND plausibly ≥ 3 future runs. Surface to the user for confirmation; never auto-create. Write "none this run" if empty.

- **name**: <kebab>
  **scope**: personal | project
  **trigger description**: <phrase / task type>
  **action**: new | update `<name>` | promote memory `<slug>`
  **steps**: 1… 2… 3…
  **why a skill not a memory**: …
  **handoff prompt for skill-creator**: <copy-paste-ready brief>

  *(leave status blank — orchestrator fills it after the approval round)*

---

**Optional sections** — add when this run produced it, delete the rest:

- **Deviations from plan** — actual ≠ plan (Task X became Y — reason)
- **Follow-ups** — append each verbatim to FOLLOWUPS.md > Open, then mirror here:
  - **item**: <one line> · **type hint**: feat | fix | … · **priority**: low | med | high
  - **consumed**: <followup-id> — landed via task <n>
- **Security findings (carry-over)** — when security.md exists (its non-blocking medium / low findings)
- **Context folded** — lines merged into `.workflow/CONTEXT.md` (repo ledger) this run
- **Decisions recorded** — rows appended to `docs/DECISIONS.md` this run

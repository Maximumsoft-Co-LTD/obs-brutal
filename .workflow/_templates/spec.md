# Spec: <title>

**ID**: `NNNN-type-slug`
**Type**: feat | fix | refactor | chore | docs | spike
**Status**: draft | approved
**Ship as**: one-drop | staged
**Open PR on ship**: yes | no
**E2E + visual**: off | on
**Parent**: none | `<parent-id>`

## Goal *(required)*

<one sentence: what we build, for whom, to what outcome. Not a metric (→ Success Criteria) or feature list (→ User Stories).>

## User Stories *(feat — required; other types: DELETE — the run's `AC#` scenarios live in its Type block below)*

Priority-ordered, each independently testable — build P1 alone and you still have a viable MVP. Acceptance scenarios carry stable ids (`AC1`, `AC2`, …) that plan / tasks / test / review reference **by id** — the `Given/When/Then` text lives here only, nowhere else.

### US1 — <title> (Priority: P1) 🎯 MVP

<one plain-language sentence: who does what, to what end>

**Why this priority**: <why this is the most critical slice>
**Independent test**: <how to verify US1 alone delivers value>

**Acceptance scenarios**

- [ ] **AC1** — **Given** <initial state>, **When** <action>, **Then** <expected outcome>.
- [ ] **AC2** — **Given** <state>, **When** <boundary / bad input>, **Then** <on-error outcome> — or `none — <default>`.

---

### US2 — <title> (Priority: P2)

<one plain-language sentence>

**Why this priority**: <value, and why lower than P1>
**Independent test**: <how to verify US2 alone>

**Acceptance scenarios**

- [ ] **AC3** — **Given** <state>, **When** <action>, **Then** <outcome>.

### Edge Cases

- What happens when <boundary condition>? → <handling> (FR-###).
- How does the system handle <error scenario>? → <handling> (FR-###).

## Type block *(non-feat — required; keep the run's Type heading, delete the rest — it carries this run's `AC#` scenarios)*

### Reproduction & Expected *(fix)*

**Repro**: <exact steps / failing input> · **Expected**: <correct behaviour> · **Actual**: <observed>

- [ ] **AC1** — **Given** the repro state, **When** <the action>, **Then** <expected behaviour holds> (regression test passes).

### Equivalence contract *(refactor)*

<one line: behaviour unchanged — the baseline that proves it>

- [ ] **AC1** — **Given** the captured baseline, **When** the restructure lands, **Then** the baseline suite passes unchanged.

### Checklist *(chore)*

- [ ] **AC1** — <checkable outcome, e.g. "dependency X at version Y, build green">.

### Docs scope *(docs)*

- [ ] **AC1** — <doc file> reflects <change>, verified against the code it describes.

### Questions & Timebox *(spike)*

**Timebox**: <hard limit> · **Deliverable**: `recommendations.md`

- [ ] **AC1** — <question> → answered with evidence in `recommendations.md`.

## Requirements *(feat — required; other types: add only when real FRs exist, else DELETE)*

### Functional Requirements

- **FR-001**: System MUST <specific capability, e.g. "add a task with a non-empty trimmed title">.
- **FR-002**: Users MUST be able to <key interaction>.

### Key Entities *(include when the feature involves data)*

- **<Entity>** — <what it represents; key attributes, no implementation>.

## Success Criteria *(feat/fix/refactor — required; chore/docs/spike: DELETE unless genuinely measurable)*

Measurable, technology-agnostic outcomes.

- **SC-001**: <measurable metric, e.g. "a first-time user adds their first item in under 5 s">.
- **SC-002**: <measurable performance / volume metric>.

## Assumptions

- <reasonable default chosen where the request was silent>.

---
*Optional — add when triggered, delete the rest: Problem · Users · User journey · Scope—Out · Glossary · NFR · Definition of Done · Constraints · References · Discovery notes · Carried-over follow-ups. (Reproduction and Timebox now live in the Type block above.) Structure + hard rules (priority, Given/When/Then with AC ids, FR/SC, no invented values, [NEEDS CLARIFICATION]) → **pm.md > Spec sections**.*

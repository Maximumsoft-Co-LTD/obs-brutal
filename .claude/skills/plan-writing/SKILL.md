---
name: plan-writing
description: Write an implementation plan that maps a spec to executable, verifiable steps with a required architecture diagram, sized for the work. Use this skill when drafting `.workflow/<id>/plan.md` in the /dev workflow (lead agent, Phase 1 step 2), OR when the user asks to "write a plan", "plan this feature", "design the implementation", "break this down into steps", "draft an RFC". Owns the size tiering (XS/S/M/L), the always-required architecture diagram (mermaid by Type), the current-state mapping (LSP-walk of existing code for non-greenfield work — required for M/L and any refactor/fix), inline AC tagging, runnable-verify rule, anti-placeholder rules, and the pre-draft self-review. Composes with the construction-fundamentals skills (programming/database/hexagonal/architecture/queue) — load the relevant construction skill first to decide *what* to build; this skill decides *how to sequence, document, and verify* what you build. Skip for throwaway scripts, single-line config edits, and conversational "what should we do about X" exchanges that haven't been spec'd yet.
---

# Plan Writing

## Why this exists

Plans fail in predictable ways: they restate the spec instead of decomposing it, they hide approach-decisions in prose, they let "TBD" and "appropriate error handling" leak through, they describe a feature without showing where it plugs into the system, they say "manually verify" when they should name a command, and they ship without acceptance-criteria traceability — so the first time AC1 actually gets verified is during review, by which point the cycle budget is already half-spent.

A plan that scales with the work, carries a diagram, ties every step to an AC, and gives every step a *runnable verify* catches those failures at plan time — minutes spent here save hours in review and test cycles. This skill is the pre-flight for the `/dev` workflow's Phase 1 step 2 (lead agent, plan mode), and the standard whenever a plan is being drafted in this repo.

## The 8 principles

### 1. Read spec.md + carried follow-ups before anything else

A plan that doesn't map back to spec is blind. Open `spec.md`, list every `Acceptance criteria` checkbox, check `Carried-over follow-ups`. Every plan step must tie to either an acceptance criterion or a carried follow-up — if it ties to neither, it's scope-creep and belongs in `FOLLOWUPS.md`, not this plan.

### 2. Set Size before drafting Steps

Size determines which sections are required, which are optional, and which should be deleted. Choose XS / S / M / L using the picker in `references/size-tiering.md`. Wrong size = either bloat (XS work in M template) or under-coverage (M work treated as S). **When borderline, prefer the larger tier** — under-covering costs cycle burn at review; over-covering costs a few skipped sections.

| Size | Trigger signals |
|------|-----------------|
| **XS** | chore / docs, 1 file, no logic change (typo, dep bump, comment cleanup). If you can describe the diff in one sentence, it's XS. |
| **S** | 1 subsystem, ≤ 2 files, simple logic |
| **M** | multi-file in one subsystem, real logic (branching, state, side effects), no contract / schema change |
| **L** | cross-subsystem, schema migration, public API contract change, or any breaking change |

### 3. Map current state before designing (non-greenfield work)

A plan that touches existing code without first stating what that code does today is gambling. The plan reads as if greenfield, the architecture diagram shows only new pieces, and the engineer discovers each load-bearing invariant by breaking it. Catching this at plan time costs a few LSP queries; catching it at review or production time costs a cycle or a postmortem.

**Required for** (write a full `Current state` section before the Architecture diagram):
- Any **M** or **L** plan
- Any **refactor** or **fix** at any size (including XS/S)

**Skip for**:
- XS/S **feat** that adds entirely new files in an isolated module with no edits to existing code
- **chore** / **docs** that don't touch live code paths
- **spike** (the spike *is* the current-state investigation — record findings in `recommendations.md`, not here)

**Fields** (in this order; cite `path#anchor` for every claim — LSP go-to-definition / find-references is the source, not prose memory; anchor format is defined in principle 5):

1. **Entry point(s)** — where execution begins for the code path being changed. Examples: HTTP route handler, CLI subcommand, cron tick, queue consumer, hook trigger, library entry function.
2. **Data / control flow** — 3–7 bullets tracing how the system behaves today through the file(s) you will touch. One hop per bullet, each citing `path#anchor`. Don't paraphrase — walk it with LSP.
3. **Callers / blast radius** — for each symbol you will rename, delete, or change the contract of: run LSP find-references and summarise. "N callers; non-obvious ones are X (`path#anchor`) and Y (`path#anchor`)". "No callers — safe to change" is a valid answer and explicitly load-bearing.
4. **Invariants the current code relies on** — silent assumptions you must either preserve or *explicitly* break: ordering, idempotency, fail-open vs fail-closed, error-swallowing semantics, transaction boundaries, single-writer assumptions, retry behaviour, timeout defaults, encoding. Each invariant is one line with a `path#anchor` citation. The new code must either preserve each or the plan must call out the break and justify it.
5. **Anti-goals** *(refactor only)* — current behaviours that intentionally stay identical, paired with the behaviour-equivalence statement in Approach. These are what the test suite will pin.
6. **Bug path** *(fix only)* — the exact route the bad data / bad call takes from input to symptom, with the wrong-step marked `← BUG`. This is the as-is of the failure; the Architecture diagram shows the fix.

For **L** tier and any non-trivial **refactor**, also draw an "as-is" mermaid diagram alongside the "to-be" Architecture diagram (principle 4). For M and smaller, prose bullets are usually enough.

Full field-by-field examples + LSP-walk technique are in `references/current-state.md`.

### 4. Architecture diagram is required, always

Pick the cheapest form that conveys the change. Mark new pieces with `★`. Diagram type defaults from the run's `Type` (full templates and worked examples in `references/diagrams.md`):

| Type | Default diagram |
|------|-----------------|
| `feat` | `flowchart LR` showing where the new piece plugs in |
| `fix` | `sequenceDiagram` of the bug path with the fix point marked, OR before/after flowchart |
| `refactor` | before/after `flowchart` or `classDiagram` of the structural shift |
| `chore` / `docs` | one line: `<file> (<change>)` OR `**Impact:** N/A — <reason>` |
| `spike` | `flowchart` with `?` on unanswered nodes |

For XS, even one line counts — keep the section. The discipline is "always have a diagram slot": habit beats exception.

When Current state (principle 3) is present, the Architecture diagram is the *to-be* — show how the existing flow changes. Don't redraw the whole as-is in the to-be diagram; that's the previous section's job.

### 5. Steps use the strict format: `action — path#anchor (new|edit|delete) — verify: <command or observable> [AC#]`

Every step has all four parts. No exceptions.

- **action** — imperative, one verb (`add`, `extract`, `wire`, `delete`, `rename`). Not "implement X" — that's a goal, not a step.
- **path#anchor** — a *re-resolvable* location, not a raw line number. Cite the **symbol** for code (`src/users.ts#getUserById`, `PaymentsClient.charge`), or a **unique quoted snippet / heading** for shell, config, markdown, or a spot inside a function with no named symbol (`dev-state-mark.sh#"command -v jq"`, `WORKFLOW.md#"## Type-aware phase matrix"`). The test: any reader must be able to re-find the target with LSP or `grep` *after earlier steps have already shifted the file* — a bare `:42` goes stale the moment a step above it edits the file, and a plan full of stale line numbers reads as untrustworthy and burns review time on cross-checking. A line number MAY be appended as an explicit write-time hint (`#getUserById (~L42)`), never as the sole handle. Use `path (new)` for new files — the `(new)` disposition already carries "new"; name the symbol-to-add when it helps. When the change mimics an existing pattern, cite the precedent by anchor (e.g., `mirror src/handlers/orders.ts#createOrder`) — a reference that survives edits beats a line range that doesn't.
- **verify** — a command (`npm test src/foo.test.ts`, `curl -s :8080/health | jq .status`, `psql -c "\d users"`) or a concrete observable (`column email_verified exists`, `feature flag returns true for opt-in users`). If you would write `manually check` or `visually inspect`, the step is too big — split it until each piece is verifiable atomically. *This is the single highest-leverage rule in this skill.*
- **[AC#]** — which acceptance criterion this step lands. A step with no AC tag is either scope-creep or evidence that the spec is missing an AC the work actually delivers — go fix the spec first.

### 6. One step → one verify; if not, split

A step that needs multiple verifications is doing multiple things. Split it. Steps map 1-to-1 to commits in spirit (atomic). The verify clause is also what `qa` will hand to its test suite, and what `engineer` will run after the step — write it as if both will literally execute it.

### 7. Type-specific rules

- **`feat`** — standard plan. Diagram = flowchart, mark `★`.
- **`fix`** — step 1 of `Steps` MUST be "write failing regression test for <bug>" encoded against `spec.md > Reproduction`. *Address the root cause, not the symptom* — if your fix step is "catch the exception" or "guard the null", ask whether the cause is upstream and document why the local fix is correct.
- **`refactor`** — one-line behaviour-equivalence statement in `Approach`: what stays identical and how it gets verified (existing suite, character test, golden file). Lean on existing tests first.
- **`chore`** — minimal plan. Skip Risks for XS.
- **`docs`** — Steps are doc edits. Files touched lists every doc file. No test planning.
- **`spike`** — `Out of scope` MUST say "no production code lands from this run — engineer writes `recommendations.md` only". Steps may be open-ended ("try option A, measure throughput at 1k req/s").

### 8. Self-review before status = draft

Before handing off, walk these scans (and `references/self-review.md` for examples):

- **Anti-placeholder** — no `TBD`, `TODO`, `???`, `appropriate X`, `as needed`, `path/to/file`, hedging modals in Steps.
- **Trigger discipline** — every section in the plan has its trigger firing. No 1-row Files touched tables, no Risks="N/A", no Dependencies="None". DELETE such sections. (Diagram is the exception — always include, one-line on XS is fine.)
- **AC sufficiency, not just coverage** — every Step still carries ≥1 `[AC#]`, but presence is the floor, not the bar. For each spec AC (its `on error / at boundary:` clause and edge sub-bullets included): the Step(s) tagged with it, taken *together*, must **fully deliver** it (not merely touch it), AND at least one of those Steps' `verify:` clause must be the AC's actual acceptance check — when the spec AC carries an `e.g.: input → expected output` example, that example is the verify target. **The error/boundary clause needs its own delivering+verifying coverage** — a plan that implements only the happy path leaves the boundary the spec explicitly called out unbuilt and unverified. A measurable `measured:` target is an AC too: its verify runs the measurement. A step tagged `[AC1]` that doesn't satisfy AC1, or an AC (or its boundary clause) whose tagged steps have no verify that proves it, is coverage on paper only — that is the gap this scan catches.
- **Section integrity** — when Alternatives appears, each rejection cites evidence (load test / incident / spike-NNN), not "feels slower". When Current state appears, every claim cites `path#anchor` (symbol or snippet, not a bare line). When diagram appears, every `★` matches a `new` in Files/Steps and vice versa.
- **Verify-per-step** — every Step's verify is a runnable command or concrete observable, never "manually check".

If any scan fails, fix the plan — do not mark `status: draft`.

## Pre-flight checklist (run top-to-bottom)

Before writing any section of plan.md:

- [ ] Read `spec.md` and its `Carried-over follow-ups`.
- [ ] Confirm `Type` matches what the spec says.
- [ ] Pick `Size` from the signal table above (or `references/size-tiering.md` for edge cases). Borderline → larger tier.
- [ ] Load the relevant construction skill(s):
  - Any non-trivial code → [[programming-fundamentals]]
  - Schema / query / migration / index → [[database-fundamentals]]
  - Backend with real domain logic → [[hexagonal-backend]]
  - System-level / cross-service decisions → [[architecture-fundamentals]]
  - Queue / broker / async worker → [[queue-fundamentals]]
  - Bug with unknown cause → [[debug-fundamentals]] *before* this skill
- [ ] Pick diagram type from `Type` (table in principle 4). Even XS keeps the section — one line is fine.
- [ ] Use **LSP first** for existing-code references (definitions, references, diagnostics) before citing `path#anchor` (symbol / snippet, not a bare line). Grep is the fallback.
- [ ] If the change mimics an existing pattern, find that pattern now and have its `path#anchor` ready to cite in Steps.
- [ ] **Map current state** (principle 3) for non-greenfield work — required when Size ∈ {M, L} or Type ∈ {refactor, fix}. Walk entry point → flow → callers (LSP find-references) → invariants with `path#anchor` citations, *before* drafting Steps. Skip only when the work is brand-new files in an isolated module.

Then draft in order: **Approach → Current state (if required) → Diagram → Steps → Files touched → (size-gated sections) → Rollback → Out of scope**.

## Section gating by Size

| Section | XS | S | M | L |
|---------|----|----|----|----|
| Approach (2–3 sent) | ✓ | ✓ | ✓ | ✓ |
| Steps (with verify + AC tag) | ✓ | ✓ | ✓ | ✓ |
| Step order line | skip | optional | ✓ | ✓ |
| Current state (principle 3) | required for refactor/fix; else skip | required when touching existing code OR refactor/fix; else skip | ✓ | ✓ (+ as-is mermaid for refactor) |
| Architecture diagram | one-line / N/A | mini mermaid (3–5 nodes) | full mermaid by Type | full + before/after |
| (Optional) Phases above Steps | skip | skip | skip | ✓ if >12 steps |
| Files touched | skip if ≤2 files | when >2 files | ✓ | ✓ |
| Alternatives considered (+ Verified line) | skip | skip | when non-obvious | ✓ |
| Risks | skip | optional | ✓ | ✓ |
| Observability | skip | when feat/fix ships runtime + new op surface | required if feat/fix | ✓ |
| Dependencies (WHEN-only) | skip unless present | skip unless present | skip unless present | when blocking handoffs exist |
| Rollback | skip (revert commit) | skip unless destructive | ✓ if destructive | ✓ runbook |
| Out of scope | skip if no creep risk | skip if no creep risk | when creep risk in implementation | ✓ |

Sections marked `skip` are **DELETED entirely** — no empty headers, no "N/A" lines. Empty sections defeat the minimum-floor principle.

### Optional Phases for L plans

When a L plan grows past ~12 steps, group them under named Phases (e.g., `### Phase 1: schema migration`, `### Phase 2: write path`, `### Phase 3: read path`). Each phase has its own ordered Steps with the same strict format. Phases let `engineer` create one `TaskCreate` per phase and one nested task per step, which makes long plans navigable without forcing them to split into separate `/dev` runs. Phases are *grouping*, not gates — the /dev workflow already gates between Phase 1 and Phase 2 at the spec-approval point.

## Relation to other skills

This skill **composes**, it does not replace:

- [[programming-fundamentals]] / [[database-fundamentals]] / [[hexagonal-backend]] / [[architecture-fundamentals]] / [[queue-fundamentals]] — these decide *what to build*. Load the relevant one **first**; their output becomes the substance of `Approach` and `Steps`.
- [[debug-fundamentals]] — for `fix` plans, run debug-fundamentals first to find the cause, then this skill to encode the fix + regression test.
- [[git-workflow]] — pairs at ship time (Phase 2 step 9). A plan's atomic Steps become atomic commits; the Type slot mirrors the commit `<type>`.

The `lead` agent in plan mode is the *caller* — it invokes this skill before drafting `plan.md`. The skill does not call `lead`.

## When to skip

Skip this skill only when:

- The user is having a conversational "what should we do about X?" exchange that hasn't been spec'd yet — that's brainstorming, not planning.
- The work is a throwaway one-off script, a single-line config edit, or anything you could describe in one sentence and ship in one commit with no design risk.
- You are reviewing an existing plan (use the `review.md` template instead) or running QA on a plan that's already implemented.

If any non-trivial code is about to land in the repo and you're about to write `plan.md`, do not skip.

## Anti-patterns (do not do these)

- **Restating the spec in `Approach`** — link to spec instead. Plan drifts; spec is the source of truth.
- **Pseudocode in Steps** — if the code is ready to write, write it during implementation, not in the plan. Pseudocode rots and never runs.
- **Hour / day estimates** — planning fallacy makes these wrong by 2–4×. Use `Size` (XS/S/M/L) only.
- **"Considerations" / "Notes" bucket sections** — every insight belongs in a section that drives action (Steps, Risks, Alternatives, Out of scope). Unbounded buckets become dump grounds.
- **Including triggered sections "just in case"** — an empty `Architecture diagram` on an XS string change, a `Files touched` table with one row, a `Risks` section that says "N/A". DELETE the whole section instead. Empty headers defeat the minimum-floor principle.
- **Inventing an Observability line because the template asks for one** — if the run doesn't ship runtime code or add a new operational surface, DELETE the section. Don't invent metric names to fill a slot.
- **`Alternatives considered` without a Verified line** — "rejected: feels slower" is a feeling, not evidence. Each rejection needs a load test, prior incident, profiling result, or spike reference.
- **Verify = "manually check" / "eyeball" / "visually inspect"** — that's not a verify. Name a command or a concrete observable, or split the step. (Anthropic's single-highest-leverage rule for working with AI coding agents is *give the agent a way to verify its work*. This rule is that rule, applied to plans.)
- **AC tag = "all"** — every step tags specific AC numbers. "All" hides which step actually lands which behaviour.
- **Symptom-patching for `fix`** — "wrap in try/catch", "guard against null" without explaining *why* the null arrives is treating the symptom. Show the root cause in `Approach`; let the fix step name it explicitly.
- **Designing for hypothetical future requirements** — if the spec doesn't ask for it, the plan doesn't plan for it. Carry the idea to `FOLLOWUPS.md` instead.
- **Designing without Current state on existing code** — if the plan touches existing files and the Current state section is missing (or empty, or written from memory instead of LSP-walked), the plan is gambling on assumptions. Either fill the section honestly or — if the work genuinely is greenfield — say so in one line under Approach so the omission is intentional, not accidental.
- **Current state that's just paraphrase, no citations** — "the hook writes state.json then exits" without a `path#anchor` is a guess. Walk it with LSP and cite. If you can't cite, you don't know it.
- **Implementing a new port/boundary without naming its interface first** — if a Step creates a new internal port (e.g. a hexagonal port between application and an adapter), the engineer will invent the method signatures and the adapter drifts from the port. Name the interface + signatures (params → return/error) in `## API / event contracts` *before* the Steps that fill them.
- **Unpinned or assumed dependencies** — a Step that says "add the `fast-csv` package" with no version, or names a package you didn't confirm exists, invites a hallucinated or typo-squatted dependency. Pin an exact existing version and make the Step's verify confirm it resolves (lockfile entry / `npm ls pkg@ver`).

## References

Pick the one that matches the friction:

- `references/size-tiering.md` — XS/S/M/L picker, edge cases (one-file state machine, mechanical sweep across 30 files, type-vs-size collisions), and per-size time budgets.
- `references/diagrams.md` — mermaid templates per Type with worked examples, when to use `flowchart` vs `sequenceDiagram` vs `classDiagram`, and L-plan two-diagram pattern.
- `references/current-state.md` — the LSP-walk technique for mapping existing code, what counts as an invariant, worked examples per Type (feat touching existing API, fix bug-path, refactor anti-goals), and the "no callers / single caller / many callers" framing.
- `references/self-review.md` — the five scans in detail with anti-placeholder regex list and extra checks for M/L plans.

If lead is drafting in plan mode and unsure which to consult, use this map: *Size unclear* → size-tiering; *which mermaid kind* → diagrams; *what does existing code do* → current-state; *plan reads "done" but feels off* → self-review.

# Plan Self-Review

Run this checklist **before** setting `Status: draft` in `plan.md`. The goal is to catch the failure modes that otherwise show up at review time and burn a cycle.

A plan that passes self-review is not "perfect" — it's "internally consistent and free of the known antipatterns". Real surprises will still surface during implementation; this checklist filters out the ones we already know how to spot.

## The five scans

Walk these in order. Each one takes ~30 seconds for an XS/S plan, ~2 minutes for L.

### Scan 1 — Anti-placeholder

Search the entire plan for these strings. Every hit is a fix-before-draft:

| Pattern | Why it's bad |
|---------|--------------|
| `TBD`, `TODO`, `???` | A placeholder is a hole. Either fill it, move it to `Out of scope`, or replace with an inline `[NEEDS CLARIFICATION: <who> — <what>]` marker in spec.md AT THE SPOT it matters. Plans should not carry placeholders past `Status: draft`. |
| `appropriate error handling`, `proper validation`, `as needed`, `where appropriate` | Vague — no engineer can implement "appropriate". State the actual behaviour. |
| `see spec`, `as discussed`, `per the design` | Forces reader to dereference. Plan should be self-contained for the slice it owns. |
| `etc.`, `and so on`, `among others` | Hides scope. List it or scope it out. |
| `path/to/file`, `foo/bar`, `<file>` | Template residue. Replace with a real `path#anchor` (symbol or unique snippet) or delete the bullet. |
| `e.g.`, `for example` *in Steps* | Steps are imperative actions, not illustrations. Move examples to `Approach`. |
| `should`, `would`, `might` *in Steps* | Steps are commitments, not hedges. "Add `getUserById` at `src/users.ts#getUserById`", not "should probably add a lookup". |
| `consider X`, `think about Y` *in Steps* | Steps are decisions already made. If it still needs deciding, it's an `Open question` in spec, not a Step. |

If any pattern appears in `Approach`, that's usually OK — `Approach` carries the *why* and may use hedging. The hard rule is **`Steps` and `Files touched` must be placeholder-free**.

### Scan 2 — Acceptance-criteria sufficiency

Coverage (the `[AC#]` tag exists) is the floor. Sufficiency is the bar: the tagged steps actually deliver the AC, and a verify proves it. Open `spec.md > Acceptance criteria`. For each checkbox:

1. Search the plan for the AC's number (`[AC1]`, `[AC2]`, ...).
2. Confirm at least one `Step` carries that tag.
3. Read those Steps. Taken *together*, do they fully deliver the AC — not merely touch it? If the connection is hand-wavy, the Step is too abstract — split it.
4. Confirm the AC's *acceptance check actually runs*: at least one tagged Step's `verify:` clause exercises the AC's behaviour. When the spec AC carries an `e.g.: <input> → <expected output>` example, that example is the verify target — the verify should produce that output for that input. A tagged AC with no verify that checks it is coverage on paper only.
5. **Cover the error/boundary clause and any measured target.** When the spec AC carries an `on error / at boundary:` line, the happy path is not enough: a Step must deliver the unhappy path AND a `verify:` must exercise it (feed the bad input / hit the limit / send the unauthorized caller, assert the recorded behaviour). When the AC carries a `measured:` perf/security/a11y target, a Step's verify runs that measurement. An AC whose boundary clause or measured target has no delivering+verifying Step is the silent-guess gap this scan exists to catch.

If an AC has no step:
- The plan is incomplete → add the steps.
- OR the AC is out of scope for this run → state it in `Out of scope` and confirm with the orchestrator/user.

If a Step has no AC tag:
- The step doesn't earn its place → delete it, OR it's scope-creep → move to `FOLLOWUPS.md`.
- OR the spec is missing an AC the work actually delivers → go back and add the AC to spec first, then re-tag.

There is no third option. Every Step ↔ at least one AC. Every AC ↔ at least one Step.

### Scan 3 — Diagram ↔ Files alignment

Walk the `Architecture diagram`:

- Every node marked `★` (new piece) → must appear in `Files touched` as a `new` row.
- Every node marked `~~strikethrough~~` (removal) → must appear in `Files touched` as a `delete` row.
- Every `new` row in `Files touched` → must appear as a `★` node in the diagram.
- Every `delete` row → must appear with strikethrough.

Edit rows (existing files modified) don't have to be in the diagram unless the edit is a structural change — but if a file is edited in a way the diagram should show (new exported function, new dependency arrow), surface it with a labeled edge or annotation.

XS plans where Diagram = `Impact: N/A` skip this scan.

### Scan 4 — Current-state coverage

Skip this scan when principle 3 says skip (XS/S feat in isolated new files, chore/docs not touching live code, spike). Otherwise walk it.

Open `## Current state` and `## Files touched` side by side. For each row in `Files touched`:

- **`new`** — no current-state coverage required (the file doesn't exist yet).
- **`edit`** — the file must appear in either:
  - the `Data / control flow` bullets (i.e., this file is in the flow we walked), OR
  - the `Invariants` list (i.e., the edit preserves or breaks a named invariant on this file).
  If the edit isn't in either, ask: do we actually understand what the current file does at the line we're editing? If yes, add the bullet/invariant. If no, walk it with LSP now — that's the gap this scan exists to catch.
- **`delete`** — must appear in the caller-walk (we know nothing else points at it) AND in the as-is flow (we know what it currently does at the call site).

Then walk `## Current state` itself:

- Every invariant has a `path#anchor` citation. "The hook fails open" is not an invariant; "the hook fails open on missing `jq` at `.claude/hooks/dev-state-mark.sh#"command -v jq"`" is.
- The caller walk gives a concrete number (0 / N / "many — listing non-obvious") for every symbol whose contract changes. "Some callers" is not a count.
- For `refactor`: the Anti-goals list ties to the Approach's behaviour-equivalence statement — both name the same invariants from opposite sides.
- For `fix`: the Bug path has a `← BUG` marker on the wrong-data step, not on the symptom step.

If any check fails, the section is paraphrase rather than mapping — re-walk with LSP and cite.

### Scan 5 — Verify-per-step completeness

Walk every Step. For each:

- Has a `verify:` clause? (Required for S/M/L; optional for XS.)
- Verify clause is a **command** (something runnable: `npm test src/foo.test.ts`, `curl -s :8080/health | jq .status`, `psql -c "\d users"`) OR a **concrete observable** (`column email_verified exists`, `feature flag returns true for opt-in users`)?
- If verify is `manually check`, `visually inspect`, `eyeball the output`, `looks correct` → reject. Either name what you're checking for (`response body includes "ok"`) or the verify isn't real.

If a Step doesn't have a clean verify, the step is doing too many things — split until each piece is verifiable atomically.

> Why this scan is the highest-leverage one: the `verify` clause is what `engineer` runs after the step, what `qa` translates into a test, and what `lead` (review mode) uses to confirm the step landed. A bad verify reaches all three later phases.

## Extra checks for M / L plans

### Alternatives section is honest (M/L feat/refactor)

If you wrote `Alternatives considered`, the rejected options must be plausible — not strawmen. "Considered X, rejected because it would be slower" without naming *why* or *by how much* is a strawman. Either give a real reason (benchmark, complexity argument, ecosystem maturity) or drop the section.

If there really *was* only one reasonable approach, write a one-line note in `Approach` saying so (`Approach is the only obvious path because <constraint>`) and skip the section.

### Rollback is real (L plans)

If `Rollback` says anything beyond "revert the commit", read it as a runbook:

- Could an on-call engineer at 2am execute it from the text alone?
- Are the steps copy-pasteable, in order, with no implicit context?
- Is `Data loss?` honest? (Almost no rollback is truly "no data loss" if the change wrote anything — be precise about *what* might be lost.)

### Dependencies are concrete

`External: some library` is not a dependency — `External: pg-listen >= 1.7.2 (for LISTEN/NOTIFY support added in 1.7)` is. `Internal: prior PR` is not a dependency — `Internal: must land after PR #482 (schema migration adds users.tenant_id)` is.

The same pinning rule applies to any package a **Step** introduces, not just the Dependencies section: an exact version that exists, with the Step's verify confirming it resolves (lockfile entry / `npm ls pkg@ver`). An unpinned or unconfirmed package name in a Step is how a hallucinated or typo-squatted dependency lands.

### Phases are coherent (L plans with optional Phases)

If you grouped Steps under Phases (>12 steps in L), each phase should have a clear name (`schema migration`, `write path`, `read path`, `consumer cutover`) and be roughly independently committable. A phase that's "miscellaneous" is a smell — either it doesn't deserve its own phase, or the steps in it should be split across the named phases.

## When to fail the self-review and rewrite

If more than 3 items across the scans need fixing, don't patch in place — that usually means the plan was drafted too quickly and the structure is off. Re-read the spec, re-pick the Size if needed, and re-draft. Faster than chasing scan hits one by one.

If Scan 4 (Current-state coverage) is the one failing, fix it *first* before re-running the other scans — most downstream gaps (vague steps, missing verifies, AC tags that don't quite fit) trace back to the planner not actually knowing what the existing code does.

## The final question

Before marking `Status: draft`, ask:

> If I handed this plan to an engineer who has never seen the spec, could they implement it without asking me anything except about ambiguities I've already flagged with inline `[NEEDS CLARIFICATION: <who> — <what>]` markers (in spec.md, at the spots they apply)?

If yes → draft.
If no → which scan caught it? Run that scan again.
If "I'm not sure" → run all five scans.

The plan is the *contract* between `lead` (planner) and `engineer` (implementer), and the *spec* that `qa` will test against. Self-review is what makes the contract stand on its own.

# Current State Mapping

Reverse-engineer the existing code *before* designing the change. This file is the deep-dive when `plan-writing > principle 3` says "map current state" and you need to know what to capture, how to walk it with LSP, and what good looks like per Type.

## Why this exists separately

The principle in SKILL.md is the rule. This file is the *technique*: which LSP queries to run, how many hops to trace, what counts as an invariant worth writing down, what to do when LSP returns 200 references, and worked examples per Type so the section doesn't reduce to "I read the file and it does stuff."

## The LSP-walk technique

You don't need to read every file. You need to *trace the change*. Do this in order:

1. **Anchor on the spec's integration points.** Open `spec.md > Constraints > Integration points` (or the equivalent — the spec's list of existing files this run will touch). Each one is a starting node for the walk. If the spec doesn't list integration points, the spec is under-specified — fix the spec first.

2. **Entry-point query.** For each integration point, ask: "where does control enter this file?" Examples:
   - HTTP route handler → which router file mounts it? `grep`/LSP find-references on the handler symbol.
   - Hook script → which hook event triggers it? Check `.claude/settings.json` or the hook config.
   - Library function → which callers exist? LSP find-references on the exported symbol.
   - CLI subcommand → which command parser registers it?
   - Cron job → which scheduler config names it?
   Cite the entry point with `path#anchor` — the symbol for code, a unique quoted snippet/heading for shell/markdown/config. A bare line number goes stale once an earlier step edits the file; an anchor stays re-resolvable with LSP or `grep`.

3. **Forward walk (3–7 hops).** From the entry point, walk *forward* through the code being touched. Each hop is: function call → next function. Use LSP go-to-definition. Stop when you reach a leaf (DB write, external API call, return value, terminal log). Cap at 7 hops — if you need more, the spec slice is too large and should split.

4. **Caller walk.** For every symbol whose contract you will change (rename, delete, change signature, change return type, change behaviour observed by callers): LSP find-references. Three buckets:
   - **0 callers** — write "no callers — safe to change". Load-bearing fact.
   - **1–3 callers** — list each with `path#anchor`. Each caller is a potential break point.
   - **4+ callers** — count, then list only the *non-obvious* ones (cross-module, test files, external consumers). "12 callers, all internal to the same file" reads differently from "12 callers across 6 modules" — both are worth recording.

5. **Invariant scan.** Re-read the forward walk and ask, for each hop:
   - Does this code rely on input arriving in a particular order?
   - Does it assume the database row already exists / doesn't exist?
   - Does it swallow errors silently (`|| return`, `try { } catch { /* ignore */ }`, `?.`)?
   - Does it open a transaction / commit / rollback? At what boundary?
   - Does it write to shared state (file, env var, global)? Single writer or many?
   - Does it have a timeout / retry default that callers depend on?
   - Does it fail-open or fail-closed on a missing dependency?
   Anything you'd answer with "yes, and the new code must preserve it" → write it as an invariant with `path#anchor`.

6. **Stop when the section is "actionable for the engineer who will implement this."** Not "everything I noticed". The goal is to give the engineer the load-bearing facts about the as-is so they can change it without breaking it — not to teach them the file.

## What counts as an invariant

A useful invariant has three properties:

- **Silent** — the current code relies on it but doesn't document it. (If it's already in a docstring or `WORKFLOW.md`, link there; don't restate.)
- **Load-bearing** — breaking it would change observable behaviour or violate a downstream assumption.
- **Citable** — you can point to `path#anchor` (symbol or unique snippet) where the assumption lives.

Examples worth writing:

- `dev-state-mark.sh#"command -v jq" — fails open on missing jq (silently exits 0); new emit calls must mirror this guard`
- `OrderService.charge — assumes idempotency key already validated upstream by the API layer; called from anywhere else, this assumption breaks`
- `users.findById — returns null (not throw) on not-found; 14 callers depend on this`
- `events.jsonl appends are non-atomic — relies on single-writer assumption (only one /dev run per workflow dir at a time)`

Examples NOT worth writing:

- "The function takes a string and returns a number" — that's the type signature, not an invariant.
- "The handler logs at info level" — unless callers depend on the exact log line, this is just behaviour, not an invariant.
- "The file is 200 lines long" — irrelevant.
- "The function uses `async/await` rather than `.then()`" — implementation style, not a contract.
- "There's a `// TODO` comment at line 42" — a known-debt marker, but not an invariant your change has to preserve.
- "The class extends `BaseService`" — relevant if subclassing matters, but on its own it's just inheritance, not load-bearing behaviour.
- "The variable is named `userId`" — naming isn't an invariant; if a caller depends on the *value* of `userId`, that's the invariant, and the name is incidental.
- "The handler returns 200" — that's just behaviour; only worth capturing if a caller depends on it being *exactly* 200 (e.g., a healthcheck probe) rather than "any 2xx".

### The one-line bar

When in doubt, ask: **would silently breaking this change observable behaviour or violate a caller's assumption?**

- Yes → it's an invariant; write it with a citation.
- No → it's just behaviour, code style, or trivia; cut it.

The bar exists because the Current state section is *load-bearing context for the engineer who will change this code*, not a file tour. A 12-bullet invariants list that includes 8 trivia items is worse than a 4-bullet list of the four things that actually constrain the change — the trivia dilutes the signal.

## Section template

Structure for the `## Current state` section in `plan.md`. Adapt per Type — fields marked *(type)* apply only to that Type.

```markdown
## Current state

**Entry point(s)**:
- `<path#anchor>` — <one-line role> (e.g., "PostToolUse hook fired by Claude Code after every tool call")

**Data / control flow** (LSP-walked):
1. `<path#anchor>` — <what this hop does> → calls `<symbol>` at `<path#anchor>`
2. ...
N. `<path#anchor>` — terminal write / return / external call

**Callers / blast radius**:
- `<symbol>` (`<path#anchor>`): N callers — <summary; list non-obvious ones with path#anchor>
- `<symbol2>`: 0 callers — safe to change

**Invariants the current code relies on**:
- `<one-line invariant>` — `<path#anchor>` <why it's load-bearing>
- ...

**Anti-goals** *(refactor only)*:
- <behaviour that stays identical>, verified by <existing test / golden file / character test>

**Bug path** *(fix only)*:
```
<input> → step1 (`path#anchor`) → step2 (`path#anchor`) ← BUG: <what goes wrong here> → step3 → <symptom>
```
```

For **L** tier + refactor, add an *as-is* mermaid diagram immediately under the section title (before the bullets), then the *to-be* in the regular Architecture diagram section.

## Worked examples per Type

### feat — touching an existing API surface

Spec: "Add `/dev-metrics` slash command that regenerates `.workflow/METRICS.md` from `events.jsonl`."

```markdown
## Current state

**Entry point(s)**:
- `.workflow/0002-feat-dev-audit-trail/events.jsonl` — written by hooks + orchestrator (no entry point in code yet; this file is *read by* the new command)
- Existing similar command: `.claude/commands/dev.md` — the `/dev` slash command, the shape we'll mirror

**Data / control flow** (today):
1. Orchestrator and hooks append JSONL events to `events.jsonl` per run (no reader yet)
2. `retro.md` aggregates "What to change" per run (read by humans, not yet by tooling)
3. `.workflow/INDEX.md` — flat list of all runs (also read by humans only)

**Callers / blast radius**:
- `events.jsonl` schema — 0 readers today; the new generator is the first. Free to design the read path against the JSONL we know the writers produce.
- `.workflow/INDEX.md` — read by orchestrator at run init (`.claude/orchestrator.md#"INDEX.md"`); generator must not break the existing read.

**Invariants the current code relies on**:
- Events are append-only — generator must read, never write `events.jsonl`. (Writers only ever append; the read path must never truncate or rewrite the file.)
- `events.jsonl` is *per run*, not global — generator must walk `.workflow/*/events.jsonl`, not assume a single file.
- `jq` is available on this developer's machine but not guaranteed everywhere; hooks fail-open on missing `jq` (`.claude/hooks/dev-state-mark.sh#"command -v jq"`). Generator script can hard-require `jq` or stay POSIX — see spec Open question.
```

### fix — bug-path with marker

Spec: "Fix: `dev-agent-guard.sh` blocks legitimate retry spawns after a stale state.json triggers the guard."

```markdown
## Current state

**Entry point(s)**:
- `.claude/hooks/dev-agent-guard.sh` — PreToolUse hook triggered before every `Agent(` invocation

**Data / control flow** (today):
1. Hook reads `tool_input` from stdin via `jq` (`.claude/hooks/dev-agent-guard.sh#"tool_input"`)
2. Checks if `subagent_type` is `orchestrator` → block (`#"orchestrator"`)
3. Checks if `subagent_type` starts with `worker-` → block (`#"worker-"`)
4. Reads `state.json` mtime via `stat -f` (`#"stat -f"`)
5. Compares against `.last_worker_return` mtime (`#".last_worker_return"`)
6. If `state.json` is newer than marker → block as "stale" (`#"-gt"`) ← BUG: the comparison uses `-gt` on mtimes, but on macOS with sub-second resolution, two events in the same second register equal, NOT newer, so the guard fires when it shouldn't (and DOESN'T fire when it should)

**Callers / blast radius**:
- Hook is called by Claude Code itself via `PreToolUse` config (`.claude/settings.json#"PreToolUse"`). 1 caller, can't be skipped.

**Invariants the current code relies on**:
- `.last_worker_return` is `touch`-ed at second granularity by `dev-state-mark.sh#".last_worker_return"` — the guard's comparison must be coarser than 1s or use a different signal.
- Hook exits 0 = allow, exits non-zero with a message on stdout = block — must preserve this contract.

**Bug path**:
```
Agent spawn (T=N) → guard reads state.json (mtime=N-2) → guard reads .last_worker_return (mtime=N-2) → -gt comparison: equal, NOT newer ← BUG: but state.json updated since spawn, so the "stale" check returns false-negative, OR a fresh spawn within 1s of a state update returns false-positive
```
```

### refactor — anti-goals

Spec: "Extract the retry-with-backoff logic that's duplicated in `PaymentsClient.charge` and `PaymentsClient.refund` into a shared `withRetry` helper, no behaviour change."

```markdown
## Current state

**Entry point(s)**:
- `src/payments/PaymentsClient.ts#PaymentsClient` — class exported as the single entry to all payment-provider calls

**Data / control flow** (today):
1. `PaymentsClient.charge` — inline `for (let i = 0; i < 3; i++) { try { return await ...; } catch (e) { ... } }` with hard-coded 3 attempts and a fixed 500ms sleep
2. `PaymentsClient.refund` — near-identical loop with 3 attempts and the same 500ms sleep, but catches one extra error class (`PartialRefundError`) before retrying
3. Both call `this.provider.<op>()` which throws on network failure; success path returns the provider response object

**Callers / blast radius**:
- `PaymentsClient.charge` (`src/payments/PaymentsClient.ts#charge`): 4 callers — `OrderService.placeOrder`, `SubscriptionService.renew`, two tests. None inspect retry count or sleep duration; safe to refactor.
- `PaymentsClient.refund` (`src/payments/PaymentsClient.ts#refund`): 2 callers — `OrderService.cancel`, one test. Same — no caller depends on retry mechanics.

**Invariants the current code relies on**:
- *Total wall-clock budget is roughly 1.5s* (3 attempts × ~500ms) — `OrderService.placeOrder` is called from inside a 2s API timeout; the new helper must default to ≤ 1.5s total or callers' timeouts will start firing first.
- *`PartialRefundError` is retried in `refund` but not in `charge`* — `src/payments/PaymentsClient.ts#"PartialRefundError"` catches it explicitly; this is intentional (partial refunds are eventually-consistent on the provider side). The helper must accept a per-call retryable-error predicate, not hard-code the exception list.
- *Sleep is `setTimeout`, not a real backoff timer* — tests at `tests/payments/PaymentsClient.test.ts#"jest.useFakeTimers"` rely on `setTimeout` being the sleep mechanism. The helper must call `setTimeout` (or expose an injectable sleeper) so the existing tests don't break.

**Anti-goals** (refactor — these MUST stay identical):
- Same number of attempts (3) with the same sleep duration (500ms) for both `charge` and `refund` for the same input → verified by the existing `tests/payments/PaymentsClient.test.ts` suite, which counts retry attempts via a mock provider.
- The `PartialRefundError`-retried-only-in-refund behaviour stays intact — verified by the existing "refund retries partial-refund errors but charge does not" test pair.
- Provider response object shape is passed through unchanged → verified by existing serialization tests at `tests/orders/checkout.golden.json`.
```

## The "no callers / single caller / many callers" framing

Caller count changes how you size the risk of the change:

| Callers | Implication | Plan response |
|---------|-------------|---------------|
| 0 | Safe to change the contract — nothing breaks. | Note it; move on. |
| 1 | Coordinate the caller change in the same plan. | The caller's update is a Step in this plan. |
| 2–5 | Each caller is a potential break. | List them all in Current state; each gets a verify step or a coverage-check step. |
| 6+ | Contract change is structural — likely belongs in a bigger plan or staged rollout. | Either bump Size to L, or split the contract change off into its own run with a deprecation path (expand → migrate → contract). |

If you find 20+ callers on a symbol the spec wants you to rename, push back on the spec — the work is bigger than a single run.

## When to draw an "as-is" mermaid

Required when:
- L tier and the change is structural (multiple files, shifted boundaries).
- Any `refactor` where the structural before/after isn't obvious from prose bullets.

Otherwise, prose bullets in Current state are enough. Don't draw an as-is diagram for an XS fix — it's noise.

When you do draw one, put it directly under the `## Current state` heading (before the bullets), label it `As-is`, and let the Architecture diagram (principle 4) be the labelled `To-be`.

## Common failure modes

- **Paraphrase without citations** — "the hook writes state.json, then exits" with no `path#anchor`. Walk it again; cite every claim.
- **Including everything you noticed** — Current state is the *load-bearing* subset, not a file tour. If a fact doesn't constrain the plan, cut it.
- **Skipping the caller walk** — "I'll find out at implementation time" — no. The caller walk is what reveals the blast radius before you commit to the change.
- **Treating the type signature as the invariant** — types are checked by the compiler; invariants are what the compiler *can't* tell you (ordering, idempotency, error semantics). Focus on the latter.
- **Bug path that's just the stack trace** — a stack trace shows *where* the symptom surfaced, not *where the data turned wrong*. The bug-path arrow with `← BUG` should mark the data-turned-wrong step, often *above* the symptom.

## When to skip Current state

The principle table is the source of truth. As a quick reference:

- XS/S `feat` adding entirely new files in an isolated module with no edits to existing code → skip.
- `chore` / `docs` not touching live code paths → skip.
- `spike` → skip (write findings to `recommendations.md` instead).

Everything else: write the section. When borderline, write it — three minutes spent now saves a cycle later.

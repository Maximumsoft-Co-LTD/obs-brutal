# Size Tiering for Plans

Size determines which sections of `plan.md` are required, which are optional, and which should be deleted. Wrong size = either bloat (XS work dragged through M template) or under-coverage (M work treated as S). When borderline, prefer the larger tier.

This file is the picker. The `lead` agent sets `Size` in the plan's frontmatter before drafting `Steps`. The picker is also the implicit answer to the question *"is this even worth a plan?"* — see [Anthropic's guidance](https://code.claude.com/docs/en/best-practices): "if you could describe the diff in one sentence, skip the plan." That sentence-test is the XS floor.

## The four tiers at a glance

| Size | Files | Logic | Contract / schema | Subsystem reach | Typical Types |
|------|-------|-------|-------------------|-----------------|---------------|
| **XS** | 1 | none | no | none | chore, docs |
| **S**  | ≤ 2 | simple | no | 1 | fix, small feat, small refactor |
| **M**  | 3–10 | real | no | 1 | feat, refactor |
| **L**  | any | real | **yes** (or breaking) | ≥ 2 | feat, refactor, fix at a seam |

"Real logic" = branching, state, side effects to design. "Simple logic" = one branch, no state. "Contract / schema" = public REST/gRPC API, DB schema, queue message shape, IPC, event payload.

## Picker — answer in order

1. **Does the change touch a public contract or schema?** (REST/gRPC API, DB schema migration, queue message format, public library function signature, breaking change to any external surface)
   → **L**. Stop.

2. **Does the change cross more than one subsystem?** (e.g., touches both the API layer and the worker layer; or two bounded contexts; or a service plus its sibling)
   → **L**. Stop.

3. **Is there real logic to design?** (branching, state machine, retry policy, ordering decision, concurrency)
   → **M** if single-subsystem (multi-subsystem already caught in step 2).

4. **Is it more than 2 files, OR does it have any logic at all?**
   → **S**.

5. **Single file, no logic, no behaviour change visible to users or callers?** (typo, dep bump, doc edit, formatter run, comment cleanup)
   → **XS**.

When two answers feel equally true (e.g., "2 files but they're trivial" vs "1 file but the logic is hairy"), pick **the larger size**. The cost of an over-sized plan on small work is a few extra optional sections you skip; the cost of an under-sized plan on real work is missed scope caught at review (cycle burn).

## Signals that override file count

File count is a *proxy*, not a rule. These signals push the size up regardless of file count:

- **One-file change that adds branching, state, or retry policy** → at least S, often M. Complexity lives in the logic, not the file spread.
- **One-file change to a public API signature or DB migration** → L. Contract changes are always L.
- **Many-file change that's pure rename / formatter / mechanical sweep** → still XS or S. Mechanical changes don't carry design risk.
- **Many-file change that's "ripple from removing one thing"** → S or M depending on the call sites' logic. Removal alone isn't M; *design* of the removal is.
- **Touches a security-sensitive path** (auth, crypto, exec, deserialise, raw SQL, file/path handling) → bump up one tier. The security review will fire anyway and the plan benefits from more documentation.
- **Introduces a queue, broker, async worker, or pub/sub topic** → at least M, often L. The contract you're committing to (delivery semantics, idempotency, retry/DLQ, ordering) needs documentation even if the code is one consumer file.

## Edge cases

### "It looks like a chore but it's actually a feature"

Example: "bump library X" sounds XS, but the new version changes the default error handling, so call sites now behave differently. That's a **feat** with **S or M** size — the behaviour change is the feature.

Rule: if user-visible behaviour changes, it's not a chore. Re-pick `Type` first, then `Size`.

### "It's one file but it's a 200-line state machine"

Logic density wins over file count. **M.** A state machine deserves a diagram, AC tags per transition, and observability around state changes — all M-tier sections.

### "It's a fix but the fix is one line"

Still run through the picker. A one-line fix to a public API is L. A one-line guard inside a function is S (with the regression test as step 1, per fix-type rule). The fix line count doesn't determine size — the *blast radius* does.

### "It's a docs-only change but it touches 30 files"

Mechanical docs change (e.g., updating an old name across all guides) → **XS**. The plan can list "find/replace + spot-check" as the entire procedure. Don't drag docs work through M ceremony.

If the docs change is rewriting a guide that *explains the system* and the system has changed in real ways → that's not docs-only; the underlying change should have its own plan, and this docs update is the doc step of *that* plan.

### "It's a spike"

Spike type is orthogonal to size. The "size" for a spike is about the *exploration scope*, not code spread. Most spikes are S or M — the lead is choosing how deep to explore, not how much code to write (none lands). Use `Timebox` in `spec.md` for the real budget.

### "It crosses DB + API + UI" (full-stack feature)

Default = single L plan, single `/dev` run. Crossing three layers is normal full-stack work, NOT a reason to split. Only split when both are true: `Ship as: staged` in spec frontmatter AND the spec lists ≥ 2 capabilities that can ship independently. See `WORKFLOW.md > Scope` for the epic-mode rule.

## What each size requires (cross-link to template)

| Section | XS | S | M | L |
|---------|----|----|----|----|
| `Approach` (2–3 sentences) | ✓ | ✓ | ✓ | ✓ |
| `Step order` line | skip | optional | ✓ | ✓ |
| `Architecture diagram` | one-line / N/A | mini mermaid (3–5 nodes) | full mermaid by Type | full + before/after |
| `Steps` (action — path#anchor — verify — [AC#]) | verify optional | ✓ | ✓ | ✓ |
| (Optional) Phases above Steps | skip | skip | skip | ✓ if >12 steps |
| `Files touched` table | ✓ | ✓ | ✓ | ✓ |
| `Alternatives considered` | skip | skip | when non-obvious | ✓ |
| `Risks` table | skip | optional | ✓ | ✓ |
| `Observability` | N/A | required if feat/fix | required if feat/fix | ✓ |
| `Dependencies` | skip unless present | skip unless present | skip unless present | ✓ |
| `Rollback` | "revert commit" line | "revert commit" or specific | ✓ if destructive | ✓ runbook |
| `Out of scope` | ✓ | ✓ | ✓ | ✓ |

`skip` means *delete the section*, not leave it empty with placeholder text. Empty sections erode the gating discipline.

## How long should a plan take to write?

Rough budget (for `lead` with the construction skill already loaded):

| Size | Plan-write time | Notes |
|------|-----------------|-------|
| XS | 2–5 min | Almost entirely template fill-in |
| S | 5–15 min | Real thinking about Steps + Diagram |
| M | 20–45 min | Alternatives + Diagram + Risks need real thought |
| L | 1–2 hr | Two diagrams + Dependencies + L-grade Rollback runbook |

If you're spending 2× the budget at any tier, something's wrong: scope grew without a Size bump, or the spec is too vague to plan against. In the latter case, go back to spec — don't paper over with a longer plan.

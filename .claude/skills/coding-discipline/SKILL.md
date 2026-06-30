---
name: coding-discipline
description: Apply the behavioral guardrails that keep an AI coding session honest — surface assumptions instead of guessing, write the minimum code that solves the problem, keep every diff surgical (touch only what the request requires), and turn the task into a verifiable goal you can loop on. Use this skill BEFORE starting any code change, large or small, in any language. Triggers on requests like "implement", "add a feature", "write this", "fix this", "refactor", "change X", "make it do Y", "clean this up" — any moment you are about to produce or edit code. Use it even when no principle is named — the trigger is any coding task where the risk is acting on a silent assumption, over-building, over-editing, or shipping with no way to verify. This is the conduct layer: it ROUTES to [[brainstorming]] when scope is ambiguous, [[programming-fundamentals]] for the code itself, /simplify for the post-hoc cleanup pass, [[git-workflow]] for commits/branches/PRs, and [[debug-fundamentals]] / qa for verification — it does not duplicate them. Skip only for pure config edits, one-line shell, or throwaway scripts.
---

# Coding Discipline

## Why this exists

Adapted from [Andrej Karpathy's observations on LLM coding pitfalls](https://x.com/karpathy/status/2015883857489522876), via the MIT-licensed [andrej-karpathy-skills](https://github.com/multica-ai/andrej-karpathy-skills) repo. The failure modes are specific, recurring, and *not* knowledge gaps — the model already knows better:

> "The models make wrong assumptions on your behalf and just run along with them without checking … don't seek clarifications, don't surface inconsistencies, don't present tradeoffs, don't push back when they should."

> "They really like to overcomplicate code and APIs, bloat abstractions … implement a bloated construction over 1000 lines when 100 would do."

> "They still sometimes change/remove comments and code they don't sufficiently understand as side effects, even if orthogonal to the task."

These are **conduct** gaps — how the session is run, not what it knows. This skill is the always-on guardrail against them. It governs *how you show up to a code task*; the domain-fundamentals skills govern *what you build*. The two compose: run this as the stance check, then the layer-appropriate fundamental.

**Tradeoff:** these guidelines bias toward caution over speed. For genuinely trivial tasks, use judgment.

## The 4 principles

### 1. Think Before Coding

**Rule:** Don't assume. Don't hide confusion. Surface tradeoffs *before* the first line, not after the mistake.

**Why:** The single most expensive LLM failure is picking one interpretation silently and running 200 lines down the wrong road. A 20-second clarification beats a 20-minute rewrite.

**How to apply:**
- State assumptions explicitly. If a request has more than one reasonable reading, name the readings — don't pick one in silence.
- If a simpler approach exists than the one implied, say so and push back.
- If something is genuinely unclear, stop and name exactly what's confusing.

**Defer to:** this is the *lightweight, always-on* version. When scope is ambiguous, oversized, or needs a real design conversation (2–3 approaches, requirement-slot interview, spec self-review), that is [[brainstorming]]'s job — hand off rather than improvising a half-interview here.

### 2. Simplicity First

**Rule:** Write the minimum code that solves the *stated* problem. Nothing speculative.

**Why:** "Flexibility" and "configurability" nobody asked for is the most common form of self-inflicted complexity. Every speculative abstraction is a guess about a future that usually never arrives, and a tax on every reader until then.

**How to apply:**
- No features, options, or config beyond what was asked. No error handling for impossible scenarios.
- No abstraction for a single call site. Inline first; extract only on the second real use.
- The test: *would a senior engineer call this overcomplicated?* If 200 lines could be 50, rewrite it.

**Defer to:** this is *write-time intent* — keep it simple as you write. [[programming-fundamentals]] owns the deeper code mechanics (complexity/Big-O, illegal states, pure core). `/simplify` owns the *post-hoc* reuse/altitude cleanup pass on a diff. Don't pre-run the simplify catalog here; just don't build the complexity in the first place.

### 3. Surgical Changes

**Rule:** Touch only what the request requires. Clean up only the mess your own change made.

**Why:** Orthogonal edits — reformatting an adjacent block, "improving" a comment, refactoring code that wasn't broken — bloat the diff, bury the real change from reviewers, and risk breaking code you didn't fully understand. Every changed line should trace directly to the request.

**How to apply:**
- Don't reformat, rename, or refactor adjacent code that isn't part of the task. Match the existing style even if you'd do it differently — consistency is a feature.
- If you spot unrelated dead code or a real bug nearby, *mention* it; don't fix it inline. Let the user decide whether it's a separate task.
- Do remove imports/variables/functions that *your* change just orphaned. Don't remove pre-existing dead code unless asked.

**Relation:** this is the write-time cousin of [[git-workflow]]'s atomic-commit discipline — a surgical diff is what makes an atomic commit and a reviewable PR possible.

### 4. Goal-Driven Execution

**Rule:** Turn the task into a verifiable success criterion, then loop until it's met.

**Why:** "Make it work" is unverifiable, so you can't tell when you're done and the user becomes the test harness. A concrete criterion lets you iterate independently and *know* you've finished.

**How to apply:**
- Restate the task as a checkable goal: "add validation" → "tests for the invalid inputs pass"; "fix the bug" → "a test that reproduces it now passes"; "refactor X" → "the existing tests stay green before and after."
- For multi-step work, write a brief plan where each step names its own verification:
  ```
  1. [step] → verify: [check]
  2. [step] → verify: [check]
  ```

**Defer to:** the *stance* lives here; the *machinery* lives elsewhere. In `/dev`, `qa` authors and runs the tests and maps them to acceptance criteria. For a bug, [[debug-fundamentals]] owns "reproduce first, fix the cause, pin it with a regression test." Don't reinvent their test strategy — adopt the goal-driven framing and route the actual verification to them.

## Pre-flight checklist

Before you touch code, four questions:

1. **Assumptions** — have I stated them, and named any genuine ambiguity instead of silently choosing?
2. **Simplicity** — is this the minimum that solves the *asked* problem, with nothing speculative?
3. **Surgical** — does every changed line trace to the request, with no orthogonal edits riding along?
4. **Goal** — do I have a concrete, checkable definition of done?

If any answer is "I don't know," resolve it before writing.

## Relation to other skills

This is a thin **behavioral wrapper**, deliberately not a competitor — it routes work to the skills that own each concern:

- [[brainstorming]] — owns the full ambiguity/scope conversation. Principle 1 is its always-on micro-version; escalate to it when the design is genuinely open.
- [[programming-fundamentals]] — owns the code mechanics. Principle 2's intent ("keep it simple") becomes that skill's concrete rules (illegal states, pure core, complexity).
- `/simplify` — owns the post-hoc cleanup pass on a written diff. Principle 2 prevents the mess; `/simplify` removes what slipped through.
- [[git-workflow]] — owns commits/branches/PRs. Principle 3 (surgical diffs) is what makes its atomic commits possible.
- [[debug-fundamentals]] and the `qa` agent — own verification. Principle 4 sets the goal-driven stance; they author the actual tests.

Run order: apply this skill *first* as the conduct check on any code task, then the layer-appropriate domain skill (`ddd-strategic` → `programming-fundamentals` → `database-fundamentals` → `hexagonal-backend` → `architecture-fundamentals` → `queue-fundamentals`, or `debug-fundamentals` for a bug). It does not replace any of them and does not re-teach their content.

## When to skip

- Pure config edits with no logic (env vars, versions, formatter rules).
- One-line shell commands or trivial REPL exploration.
- Throwaway scripts you'll delete within the hour.

For anything else — even the "small" feature, even the "quick" fix — the four principles apply.

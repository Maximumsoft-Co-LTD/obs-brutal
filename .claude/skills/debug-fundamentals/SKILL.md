---
name: debug-fundamentals
description: Apply the high-leverage fundamentals that govern any non-trivial debugging session — reproducing reliably, reading what the system actually said, separating facts from assumptions, bisecting the search space, changing one thing at a time, tracing causality through the right layer, and fixing the cause with a regression test. Use this skill BEFORE you start guessing at fixes, sprinkling try/catch, or "trying things to see what happens" — for any bug, crash, regression, flaky test, performance cliff, or unexpected production behavior, in any language or stack. Triggers on requests like "fix this bug", "this is broken", "why is X happening", "debug this", "this test fails sometimes", "the request returns 500", "this used to work", "track down this regression", "why is it slow", "why did this break in prod", "investigate this incident". Use it even when the user doesn't say the word "debug" — the trigger is any unknown-cause failure where the next move is investigation, not feature work. The skill provides a 7-principle pre-flight checklist with worked examples, plus reference files on reproduction, bisection, instrumentation, and distributed-system debugging. Skip only for one-line typo fixes where the cause is already on the screen, or for greenfield feature work where nothing is broken yet.
---

# Debug Fundamentals

## Why this exists

Debugging is where engineering hours actually go. Most stuck debugging sessions are not stuck for lack of cleverness — they're stuck because the engineer skipped a fundamental and is now iterating on a hypothesis that was never grounded in evidence. The classic failure modes look like this:

- Changing five things at once, the bug goes away, nobody knows why, it comes back next week.
- "Adding a try/catch" so the symptom stops without finding what produced it — the program now silently continues with bad state.
- Reading the *first* line of the stack trace, guessing the rest, fixing the wrong layer.
- Re-running the failing test until it passes "to unblock the build," leaving a real race in production.
- Assuming the framework is broken before assuming the code is.

The principles below are language-, stack-, and decade-agnostic. They're what separates an hour of debugging from a day. Apply them *before* you start changing code, not after the third "let me try…" round.

## The 7 principles

Each principle has a one-line rule, a *why*, and a worked example. Apply them roughly in order — the early ones unlock the later ones.

---

### 1. Reproduce reliably before you diagnose

**Rule:** Find the smallest, most reliable trigger before you start hypothesizing about causes. If you can't make the bug happen on demand, every "fix" you ship is a guess you can't verify.

**Why:** Every diagnosis is a hypothesis, and a hypothesis you can't test is just a feeling. A repro is the only loop that lets you say "with this change, the bug is gone" and *know* it. A reliable repro also forces the bug to commit to a shape — it stops being "sometimes it 500s" and becomes "it 500s when `order.total === 0`", which is a debuggable thing.

**How to apply:**
- Capture the exact input, environment, time, user, and request that produced the failure. "It didn't work" is not a repro; `curl -X POST .../orders -d @failing-payload.json` is.
- Strip the repro down. Remove every step that, when omitted, still produces the bug. What's left is the actual trigger.
- For intermittent bugs, find what increases the rate. Concurrency? A specific clock minute? A cold cache? A particular row in the database? Increase that knob until the bug is reliable, then debug.
- If a real repro is genuinely impossible (rare in practice — usually you just haven't tried hard enough), increase observability *in production* until the next occurrence carries enough evidence.
- See `references/reproduction.md` for the deeper guide on minimization and flaky-bug strategies.

**Example:**
```
Bad path:  "Users report orders sometimes don't go through." → engineer reads code, guesses, deploys a retry, ships.
Good path: Pull failing request from logs → replay against staging → fails 1/20 times → run in a 100-iteration loop → fails 5/100
           when concurrent with a status webhook → now you have a race to find.
```

---

### 2. Read what the system actually said

**Rule:** Read the full error, the full stack trace, the full log line — *before* you form a theory. Most bugs name themselves in the first or last frame.

**Why:** It is astonishingly common to skim the top of an exception, pattern-match it to a familiar shape, and start "fixing" a bug the system never reported. The error message is the system's testimony about what it saw at the moment of failure. Skip it and you debug your imagination instead.

**How to apply:**
- Read the whole stack trace, not just the top. The top frame is *where it crashed*; the cause is usually three to ten frames down, in your code, not in the framework.
- Read the error *text*. `NullPointerException at User.getEmail()` and `NullPointerException at OrderItem.getEmail()` look the same and are completely different bugs.
- Note the timestamp, the request id, the host, the version. "This started at 14:32" plus "deploy at 14:30" is half the investigation.
- For UI bugs: open the devtools network tab and the console *first*. The browser already told you what happened.
- For a silent failure: turn up the verbosity. Default log level is rarely enough during a real debug session.

**Example:**
```
Stack trace ends with:
  at processOrder (orders.ts:84)
  at handler (api.ts:23)
  TypeError: Cannot read properties of undefined (reading 'currency')

Bad: "must be a null check thing, add `?.` to all the property reads."
Good: orders.ts:84 dereferences `order.total.currency`. So `order.total` is undefined. Why? → grep the writes to `order.total` →
      one write path sets it from `req.body.amount`, which is a number, not the `Money` object the rest of the code assumes. Found.
```

---

### 3. Separate facts from assumptions

**Rule:** Before each new step, write down (literally or mentally) what you *know* (verified) versus what you *assume* (plausible but unchecked). Most stuck sessions are stuck on a wrong assumption everyone forgot was an assumption.

**Why:** Bugs hide in the gap between "obviously X is true" and "X is actually true." Which version is deployed? Which config file is loaded? Which database is the app pointed at? Is this even the branch the bug is on? Every long debugging story has a moment of "wait — is it even running the new code?" Catching that earlier saves hours.

**How to apply:**
- For each load-bearing belief, ask: *how do I know that?* If the answer is "it's always been that way" or "the docs say so" or "obviously," it's an assumption, not a fact.
- Verify the cheap ones immediately: print the config, print the version, print the input as the function actually received it, print the env var. One log line beats one hour of theorizing.
- Watch for the words "should," "must be," "can't be." Those are flags. "The cache *must be* invalidating" → go check.
- Especially suspect: data shape (is it really a `number`, or a `string` that looks like one?), nullability (is this field actually always present?), ordering (does this really arrive before that?), identity (are these two object refs really the same one?).

**Example:**
```
Assumption: "The webhook handler is idempotent — we check the dedup table."
Fact-check: Read the handler. The dedup check runs *after* the row insert. Under concurrency, two webhooks both miss the check
            and both insert. Bug found by interrogating the assumption instead of accepting it.
```

---

### 4. Bisect the search space

**Rule:** Halve the suspect region with each step. Don't read code linearly hoping to spot the bug — drive a wedge into the search space and split it.

**Why:** A bug lives in some specific subset of (commits × files × lines × inputs × configs × dependencies). Reading sequentially is `O(n)`. Bisecting is `O(log n)`. On a 200-commit regression window, that's 7 steps versus 200. The same logic applies to "which of these 40 fields broke serialization" or "which middleware is dropping the header."

**How to apply:**
- *When did it break?* → `git bisect` between a known-good and known-bad commit. Automate the test if you can; the bisect runs itself.
- *Where in the code path?* → drop a print/log at the midpoint of the suspect call stack. If the value is still right there, the bug is downstream; if it's already wrong, upstream. Halve again.
- *Which input field?* → start with a minimal payload that works; add half the failing payload's fields; if it now fails, the trigger is in the half you added. Repeat.
- *Which dependency?* → pin all versions, then bisect by half-reverting the lockfile diff.
- *Which feature flag/config knob?* → flip them in pairs.
- See `references/bisection.md` for `git bisect` mechanics, automation, and the dual to "binary search" when the search dimension isn't ordered.

**Example:**
```
"It worked Tuesday, breaks today." Tuesday's commit = abc123. Today's = z999. 200 commits between.
git bisect start z999 abc123 → checkout midpoint → run the repro → good/bad → ~8 steps → "the bug arrived at commit 47fa1c."
That single commit is now your entire search space.
```

---

### 5. Change one thing at a time

**Rule:** Vary one variable per experiment. If you change three things and the bug disappears, you've learned nothing — you've just added three new unknowns to the codebase.

**Why:** Causality in software is already hard; pile-on changes make it impossible. The scientific method works for the same reason in code as in chemistry: a clean experiment isolates the cause. The corollary is just as important — *also* revert your scaffolding (print statements, debug flags, hacked-up configs) once you're done. Otherwise you're shipping a debug build to production with extra surface area for the next bug.

**How to apply:**
- One hypothesis, one change, one observation. Write the prediction down before you run the experiment — "if I change X, I expect Y." If you got Z, the hypothesis was wrong; don't quietly adopt the new theory without saying so.
- When you find what fixes it, *put back the bug* and verify it returns. Then put the fix back and verify it's gone. This proves the fix actually causes the cure — not some unrelated coincidence.
- Throw away all the failed experiments. Don't leave the "for safety" try/catch, the "just in case" retry, the commented-out block. Each one is a future debugging trap.
- Commit your debug instrumentation separately so you can drop the commit cleanly.

**Example:**
```
Bug: API returns 500 intermittently.
Bad: "Bumped the DB pool, added a retry decorator, increased the timeout, swapped to a different JSON lib." Bug goes away. Why? Unknown.
     Returns 2 weeks later. None of the four changes can be safely reverted because nobody knows which was the fix.
Good: Bumped DB pool → no change. Reverted. Added retry → masks the symptom, not a fix. Reverted. Increased timeout → no change.
      Reverted. Swapped JSON lib → bug gone. Confirmed: the original lib mis-parses a specific Unicode escape under load. Real fix.
```

---

### 6. Trace causality through the right layer

**Rule:** Walk the causality chain from the symptom back through the stack — UI → API → service → DB → infra — and stop at the first place the data turned wrong. Fix it there, not where it surfaced.

**Why:** Bugs surface far from where they originate. A wrong number on a dashboard might be a frontend formatter, a serializer dropping precision, a service rounding too early, a SQL `INT` truncation, or a clock skew on a cron host. "Fixing" the dashboard formatter when the cause is the SQL truncation hides the bug everywhere else that reads the same column. Every layer above the root cause is downstream of the lie — patching there just teaches the lie to more places.

**How to apply:**
- Start at the symptom. Capture the actual wrong value and the expected one.
- At each layer boundary, ask: "what does this layer receive, and is it already wrong here?" Log it if you can't tell.
- The first layer where the value is already wrong is the layer above the bug. Look there.
- Resist the urge to "patch the display" once you find a workaround higher up. Yes, even with a deadline. Note the real bug, schedule it, don't lose it.
- Pay attention to layer-boundary translations: serialization, type coercion, units (cents vs dollars, ms vs s), timezones, encoding. Most cross-layer bugs live there.

**Example:**
```
Symptom: invoice PDF shows $0.10 instead of $10.00.
Trace:
  PDF renderer receives `amount: 0.10`. ← already wrong here.
  API response from invoice service: `amount: 0.10`. ← already wrong here.
  DB query result: `amount_cents: 1000`. ← right here.
  → service does `amount_cents / 10000` instead of `/ 100`. Bug is at the service/db boundary. Fix at the source; every downstream
    consumer is now correct without changes.
```

---

### 7. Fix the cause; prove it with a regression test

**Rule:** Once you know the cause, fix it where it lives, then write a test that fails without the fix and passes with it. Commit both together.

**Why:** The fix and the test are two halves of the same proof. The fix changes behavior; the test pins it. Without the test, the bug can come back the moment someone "refactors that ugly bit" — and it will, often by the same engineer six months later. A bug that has happened once is likely to happen again; you cheaply buy a permanent guarantee with one test.

**How to apply:**
- Write the test first if you can — it's the cleanest verification that you actually fixed *this* bug and not a different one.
- The test should target the specific failure shape (the exact race, the exact input, the exact boundary), not just "the broader feature works." Otherwise it'll go green from any unrelated change and stop catching the regression.
- If the cause is a missing invariant, prefer encoding the invariant in the types or schema (see [[programming-fundamentals]] principle 2) over relying on a runtime test. A type-system fix is enforced for free, forever.
- If the bug is in shared state (DB, queue, cache), the regression test usually wants a real instance — mocks lose the very interaction that produced the bug. (See [[database-fundamentals]] and [[queue-fundamentals]] for the patterns.)
- Write the postmortem note even for small fixes — what was the wrong assumption that allowed this bug to exist? That's the actual lesson, not the diff.

**Example:**
```
Cause: duplicate webhook delivery, dedup check ran after insert under concurrency.
Fix: move dedup into a `UNIQUE` constraint on `(provider, external_id)` — illegal state now unrepresentable in the schema.
Test: integration test that fires two identical webhooks concurrently against a real Postgres; asserts exactly one row inserted.
      Fails on the old code, passes on the new.
```

---

## Pre-flight checklist

Before you start changing code to fix a bug, run through these in your head:

1. **Repro:** Can I trigger this on demand, with the smallest possible input?
2. **Evidence:** Have I read the full error message, stack trace, and relevant logs — not just the first line?
3. **Facts vs assumptions:** What's verified, what's assumed? What's the cheapest assumption I can convert to a fact right now?
4. **Search space:** Have I bisected (commits, code path, input, dependency) instead of skimming linearly?
5. **One variable:** Am I changing exactly one thing per experiment, with a prediction written down?
6. **Right layer:** Where does the data first turn wrong? Am I about to fix the symptom layer instead of the source layer?
7. **Proof:** Once I think I've fixed it, do I have a test that fails without my change and passes with it?

If any answer is "I don't know," stop and find out before continuing. The cost of a wrong "fix" is days; the cost of one more verification step is minutes.

## When to skip this skill

- One-line typo or obvious off-by-one where the cause is already visible on the screen and the fix is trivial.
- Greenfield feature work — nothing is broken yet; use [[programming-fundamentals]] instead.
- Pure config edits whose effect is obvious and reversible (formatter rules, package versions). These are not bugs; they're knobs.

For anything else — yes, even "I think I see the bug," even "this is just a quick fix" — these fundamentals apply. The bugs that eat days are almost always the ones where the first instinct was "I see it, I'll just…"

## Relation to other skills

Debug fundamentals are the *recovery* sibling to the construction-time fundamentals. They compose, they don't compete:

- [[programming-fundamentals]] — the layer below. Most bugs come from violations of these (illegal states, impure functions, swallowed errors, accidentally quadratic loops). When you find the cause, fixing it almost always means *applying* a programming-fundamentals principle — most often "make illegal states unrepresentable" so the bug can't return.
- [[database-fundamentals]] — when the bug crosses into persistent state: bad data, lost updates, deadlocks, slow queries, broken migrations. Use this skill to *find* the bug; use database-fundamentals to *fix* the schema, query, or transaction that produced it.
- [[hexagonal-backend]] — bugs that look like "the wrong adapter," ports leaking infra concerns into the domain, untestable code paths. Often the fix is moving a side effect to the right layer; this skill helps locate it, hexagonal-backend tells you where it belongs.
- [[queue-fundamentals]] — distributed/async bugs (duplicate delivery, lost messages, ordering, poison pills, ack/retry traps) are the dominant production-debug category. This skill's reference on distributed debugging points back at the queue principles for the fix shape.

Run order when multiple apply: use *this skill first* (find the actual cause), then the construction skill that owns the fix layer.

## Reference files

Deeper guides for individual moves. Read the one that matches what you're stuck on; you don't need to read them all upfront.

- `references/reproduction.md` — building a reliable repro, minimizing it, taming flaky failures, getting a repro out of production-only bugs.
- `references/bisection.md` — `git bisect` (manual and `run`-mode), binary search through code paths, inputs, configs, dependencies.
- `references/instrumentation.md` — strategic logging, structured logs, breakpoints, tracing, `strace`/`dtrace`/eBPF — what to print, where to put it, and how to read it back.
- `references/distributed-debugging.md` — correlation IDs, distributed traces, races, ordering, retries, partial failures, time-skew bugs.

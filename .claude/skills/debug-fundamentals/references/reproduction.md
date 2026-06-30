# Reproduction

A bug you can't reproduce is a bug you can't debug — every "fix" is a hope, not a verified change. This guide is about getting from "it happened to someone, sometimes" to "I can make it happen on demand, in 30 seconds, locally."

## What a real repro looks like

A repro is a procedure such that:
1. **It's specific.** Exact input, exact environment, exact version, exact command.
2. **It's deterministic enough to iterate.** If it fails 1 in 1000 runs, that's not yet a repro — that's a flake hunt. Get the rate up before debugging the cause.
3. **It's cheap to run.** A repro that takes 20 minutes per attempt is a debugging accelerator running in reverse. Get the cycle below 60 seconds before you start changing code.
4. **It's standalone.** A repro that needs prod data, your laptop's `~/.aws`, and yesterday's deploy is fragile. Bake the inputs in.

The artifact at the end of this stage is usually one of: a failing test, a one-liner `curl`, a script, or a saved request payload. Pick whatever the bug fits.

## The repro funnel

Work top-down through these. Stop at the first one that gives you a reliable trigger.

### 1. Reproduce in production-equivalent

If the bug is in prod, your first repro target is staging or a prod replica. Same data shape, same configs, same versions. *Most* prod-only bugs disappear at this stage — meaning the cause is environmental (data, config, version skew) rather than code. That's a *fact*, not a failure: now you know where to look.

If it still reproduces in staging, push down.

### 2. Reproduce locally

Run the same code on your machine against the same dependencies. Docker Compose, devcontainer, whatever the project uses. If it stops reproducing here, the cause is in something your local setup *doesn't* have — usually load, scale, real data, or a real dependency you mocked.

If it still reproduces, push down.

### 3. Reproduce in a test

Write a failing test that triggers the bug. This is the gold standard — fast, deterministic, replayable on CI, doubles as the regression test once you fix the bug.

```ts
test('order with zero-priced item does not crash discount math', () => {
  const order = makeOrder({ items: [{ price: 0 }] })
  expect(() => applyDiscount(order)).not.toThrow()  // currently throws DivByZero
})
```

If you can't write a test because the bug needs the database, the network, or a real queue — that's fine; write an *integration* test against a real instance. Don't reach for mocks; the mock is exactly where the bug isn't.

## Minimizing the repro

A 500-line failing payload teaches you nothing. A 3-field failing payload teaches you the trigger. Shrink by removing one piece at a time:

- Remove a header / query param / field. Re-run. If still fails, leave it out. If now passes, the trigger touches that piece.
- For long inputs (logs, files, requests), binary-search: chop the input in half. Which half still fails? Keep that. Chop again.
- For multi-step flows (request A, then B, then C breaks), drop one step at a time. Does C still break without A? Without B? The minimal sequence is your repro.

End state: the smallest input/sequence/state that still triggers the bug. Often that minimization itself reveals the cause — "oh, it's specifically when `total === 0`."

## When the bug is "flaky"

A flaky bug isn't random. It's deterministic — you just don't know all the inputs yet. Some hidden variable is changing between runs. Find it.

Common hidden variables, by frequency:
- **Concurrency / interleaving.** Two requests, two threads, two goroutines racing. Add load. Run the repro in a tight loop with `seq 100 | xargs -P 20 -I _ <repro>`. Watch the rate climb.
- **Time.** A test that fails at 23:59 UTC is a timezone or date-rollover bug. Pin the clock.
- **Order of map/set iteration.** Most languages don't promise order. If your output depends on it, it'll flake forever. Sort or use ordered collections.
- **Test isolation.** A test that fails only when run after another test is depending on global state — DB rows, env vars, a singleton. Run it alone; run the suite in reverse; bisect the order.
- **Network / DNS.** Retries mask flakiness. Disable them and watch the underlying failure.
- **Floating point.** `0.1 + 0.2 !== 0.3`. If the bug involves money or comparisons, suspect this.
- **Random seeds.** Pin them. Most "random" failures are deterministic given a seed.
- **Cache state.** Cold vs warm cache reproduces differently. Clear it. Or fill it.
- **Resource limits.** File descriptors, memory, connection pools. Bug shows up at hour 4 of a soak test? You're leaking something.

Flow: list the variables you *think* are constant → pick the most suspicious → vary it deliberately → does the rate change? Repeat.

## Getting a repro out of a production-only bug

Sometimes the bug really won't repro outside production. Often this is because production has data, scale, or traffic patterns you can't simulate. The play is to make production carry the evidence:

1. **Log the inputs.** When the failure handler fires, log the full inbound request (sanitized), the user id, the timestamp, the request id, the version, the host.
2. **Capture the state.** Log the value of any field involved in the failure path, *before* it's used. If `order.total.currency` blows up, log `order.total` and `typeof order.total` upstream.
3. **Wait for the next occurrence.** With the right instrumentation, one more occurrence is one more repro. Don't waste it.
4. **Replay it locally.** Once you have the payload from logs, send it as a test or a `curl`. Almost always reproduces. The remaining cases are usually concurrency or time-dependent — go back to "hidden variables" above.

When even logging the inputs is too sensitive (PII, compliance), log the *shape* — types, lengths, presence of fields — not the values. That's often enough to identify the trigger.

## Common pitfalls

- **"It reproduces sometimes" stops being good enough below ~80% rate.** Stop calling it a repro. Either find the missing variable or accept it's a flake hunt and instrument prod.
- **Re-running the failing test until it passes is not a fix.** It just teaches your CI to hide the bug.
- **Repros that need your dev branch and your laptop are not portable.** Bake them into the repo as a test or a script so the next engineer doesn't start from zero.
- **Don't lose the repro after the fix.** Promote it into the test suite. That's principle 7 in the main skill — the proof that the fix sticks.

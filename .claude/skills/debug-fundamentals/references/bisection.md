# Bisection

Bisection is the single highest-leverage debugging technique. Reading code linearly is `O(n)`. Bisection is `O(log n)`. On a 200-commit regression, that's 8 steps versus 200. The trick is recognizing that almost *any* search space can be bisected — not just commits.

## The general shape

Bisection works when three things are true:

1. There is a **search space** (commits, lines, fields, items, configs, versions).
2. You can **test any point** in that space and get a binary "good" or "bad" answer.
3. The answer is **monotonic** — at some boundary it flips from good to bad and never flips back.

Whenever you can frame the problem that way, you can find the boundary in `log₂(n)` steps.

## `git bisect` for regressions

The canonical case. "It worked Tuesday, broken today."

### Manual mode

```bash
git bisect start
git bisect bad                    # current commit is broken
git bisect good <known-good-sha>  # this commit was fine
# git checks out the midpoint
<run the repro>
git bisect good   # or: git bisect bad
# git checks out the next midpoint
# repeat until: "<sha> is the first bad commit"
git bisect reset
```

### Automated mode (preferred)

If you have a script that exits 0 for "good" and non-zero for "bad", git can drive the whole bisect:

```bash
git bisect start <bad-sha> <good-sha>
git bisect run ./scripts/repro.sh   # exit 0 = good, non-zero = bad, 125 = skip
git bisect reset
```

Now you can go to lunch. Come back to a one-commit verdict.

### Bisect script anatomy

```bash
#!/usr/bin/env bash
# repro.sh — exit 0 if the bug is absent, 1 if present, 125 if untestable here
set -u
make build > /dev/null 2>&1 || exit 125   # build broke at this commit, skip
./run-repro.sh > /dev/null 2>&1
case $? in
  0) exit 0 ;;   # repro passed → bug not present → "good"
  1) exit 1 ;;   # repro failed → bug present → "bad"
  *) exit 125 ;; # something weird, skip
esac
```

**Exit code 125 is special** — it tells `git bisect` to skip this commit (build broken, infra missing, etc.) rather than mark it good or bad. Use it whenever the test is *uninformative*, not when the bug is *absent*.

### Common bisect pitfalls

- **The repro must be deterministic.** A flaky test inside `git bisect run` will mark random commits bad. Get the repro rate to ~100% before you start.
- **Bisect inverted.** If the bug was *introduced by a fix* (you want to find when it was *fixed*), swap good and bad. Or use `--term-old`/`--term-new`.
- **Submodules / lockfiles drift.** Bisecting across a lockfile update may try old code against new deps. Either rebuild deps each step or pin them.
- **Long build times kill bisect velocity.** It's `log₂(n)` *full builds*. Cache, or test against a pre-built artifact when you can.
- **Refactor commits.** Massive moves can break the build at intermediate commits — `git bisect skip` them and keep going.

## Bisecting inside one commit — where in the code?

Once you know the commit, you may still have a 500-line diff to find the bug in.

Approach: **print at the midpoint of the suspect call stack.**

```ts
function chargeOrder(order) {
  validate(order)
  console.log('after validate:', order.total)   // ← midpoint instrumentation
  const taxed = applyTax(order)
  // ...
}
```

- Is `order.total` already wrong by the midpoint? Bug is upstream of the log. Move the log earlier; repeat.
- Is it still right? Bug is downstream. Move the log later; repeat.

Each step halves the suspect region of code. Three or four logs almost always pinpoint the line.

When the stack is shallow but the function is long: `console.log` at line N/2 inside the function. Same idea, smaller scale.

For compiled languages where adding prints is slow: do it at function boundaries first, narrow to one function, then add finer logging only in that function.

## Bisecting inputs — which field?

You have a 40-field payload that breaks. A 5-field minimal payload that works. Find which subset triggers the bug.

```python
def fails(payload): ...

# Start with what works
base = minimal_passing_payload
extra = failing_payload_fields_not_in_base   # list of (key, value) pairs

# Bisect on the extra list
lo, hi = 0, len(extra)
# Invariant: base + extra[:lo] passes; base + extra[:hi] fails
while hi - lo > 1:
    mid = (lo + hi) // 2
    if fails({**base, **dict(extra[:mid])}):
        hi = mid
    else:
        lo = mid
# The triggering field is extra[lo]
```

Same pattern works for: large input files, long arrays, complex nested objects, sequences of API calls.

## Bisecting dependencies — which version?

A package upgrade broke something. You don't know which one.

- Pin everything in the lockfile.
- Revert lockfile entries in halves. Test each half.
- Whichever half still fails contains the culprit. Repeat.

For a single package suspected: `git bisect` *its* repo between the old working version and the new broken one. Library authors test their own bisects; usually they cooperate.

## Bisecting flags / configs

Twenty feature flags. One combination triggers a bug. Tedious manual exploration → bisect:

- All flags **on** → bug present.
- All flags **off** → bug absent.
- Turn off half. Bug still present? → it's in the other half (which is on). Now bisect within "on."
- Bug now absent? → it was in the half you turned off. Bisect within "off."

`log₂(20) ≈ 5` flips instead of 2^20 combinations.

## When bisection doesn't apply

- **Multiple bugs at once.** The good/bad answer is no longer monotonic — some commits are bad for reason A, others for reason B. Untangle by changing your repro to target only one bug at a time.
- **No known-good point.** "It's always been broken" — there's nothing to bisect *against*. Drop back to logging and search the call stack directly.
- **The state of the world matters, not the code.** Some bugs are about *data*, not commits. The right bisect is on the data (which row breaks? which user?), not the git history.

## The mindset

Every time you're about to read a 500-line file or a 200-commit log looking for the bug, ask: *is there a binary question I could ask to halve this?* The answer is almost always yes. The discipline is recognizing it before you've already wasted an hour scrolling.

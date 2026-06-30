# Coverage Recipes

Coverage must measure **system coverage**, not **file coverage**. "workflows 10/10 documented" does NOT mean
100% — it means the 10 docs that exist are current. The real question is: *of the things in the system that
SHOULD be documented, how many are?* Always compute a ratio of `documented / actual-in-code`, and **fabricating
the denominator is worse than reporting a gap** (instrumentation-gates principle: no invented precision).

For each metric: derive the denominator from source (grep/list), the numerator from the docs, report the ratio
**plus the names of the gap** so it's actionable, not just a number.

## Entry-point coverage
```
documented routes / actual registered routes
```
Denominator: list real routes (e.g. grep route registrations in `routes/`, Gin `.POST(`/`.GET(`).
Numerator: routes that appear in some workflow doc. Report the un-covered route paths.

## Consumer coverage  *(if the service consumes a queue/stream)*
```
documented consumers / actual consumers
```
Denominator: grep message-handler/subscriber registrations. (hash-central: 0 — synchronous HTTP only → N/A.)

## Cron / scheduler coverage
```
documented jobs / actual scheduled jobs
```
Denominator: grep the cron/scheduler registration (e.g. `StartServiceCronjob`, ticker loops).

## Business-rule coverage
```
rules with ≥1 backing test / total documented rules
```
Surfaces rules asserted but unverified — those are the fragile ones. Report rule IDs with no test.

## Behavior-test coverage
```
acceptance behaviors with a mapped test / total behaviors in specs+workflows
```

## Config/env coverage
```
env vars documented for operators / env vars read in production code
```
Denominator: `grep -rhoE 'os.Getenv\("([A-Z0-9_]+)"\)' <prod dirs> | sort -u` minus test-only vars.
(hash-central run: 38/38 after update — with 2 test-only vars excluded *and named*, not silently dropped.)

## Reporting shape

| Metric | Ratio | Gap (named) |
|--------|-------|-------------|
| Entry-point | 24/27 | `POST /api/con-proxy-test`, `POST /webhook-by-proxy`, … |
| Business-rule (tested) | 5/7 | BR-PX-04, BR-DG-02 untested |
| Config/env | 38/38 | — (2 test-only excluded: SLIP2GO_LIVE_TEST, SLIP2GO_SAMPLE_QR) |

If a denominator can't be derived from source with confidence, report the metric as **directional / UNKNOWN
denominator** — never publish a clean percentage you can't defend.

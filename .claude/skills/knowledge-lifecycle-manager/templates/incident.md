---
source_commit: <sha at time of incident / fix>
generated_at: <YYYY-MM-DD>
generator: knowledge-lifecycle-manager
confidence: HIGH | MEDIUM | LOW
status: CURRENT | STALE | REVIEW | DEPRECATED
---

# Incident: <id — short title>

> Use the repository's **existing** incident convention for id and location (e.g. `.workflow/<run-id>/`,
> `FOLLOWUPS.md`, `incidents/INC-xxxx.md`). Do not invent a new convention — discover first.

<!-- MANAGED_START -->

## Problem
<What broke, in one sentence.>

## Symptoms
<Observable signals: log tags, error rate, user-facing message, dashboard panel.>

## Root Cause
<The actual cause, traced to a symbol/commit. Not the symptom.>

## Impact
<Blast radius: which tenants/flows, how long, money/data effect, severity.>

## Fix
<What changed. Cite the commit / run id. Note rollback mechanism if any (env kill-switch etc.).>

## Regression Test
<The test that now pins this so it cannot recur. If none exists, mark MISSING — that is itself a finding.>

## Lessons Learned
<What this teaches about the system; link to the business rule or risk-map entry it should reinforce.>

## Evidence
```
Code Symbol:  <pkg.Func at fault / fixed>
Commit / Run: <sha or run-id>
Test:         <regression test, or MISSING>
Workflow:     <affected workflow>
Confidence:   HIGH | MEDIUM | LOW
```

<!-- MANAGED_END -->

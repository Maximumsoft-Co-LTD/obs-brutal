---
source_commit: <sha>
generated_at: <YYYY-MM-DD>
generator: knowledge-lifecycle-manager
confidence: HIGH | MEDIUM | LOW
status: CURRENT | STALE | REVIEW | DEPRECATED
---

# Testing Knowledge: <area / flow>

> One file per testing concern (test-strategy, characterization, behavior, regression, integration,
> contract). Records the **safety net that actually exists** — never claim coverage without a named test.

<!-- MANAGED_START -->

## Scope
<Which flow / module this testing doc covers.>

## Coverage Status
| Test concern | Status | Test(s) / Location | Notes |
|--------------|--------|--------------------|-------|
| Characterization (locks current behavior) | EXISTS / MISSING / STALE | <test name> | <what it pins> |
| Behavior (business flow) | EXISTS / MISSING / STALE | <test name> | <BR mapped> |
| Regression (known bugs) | EXISTS / MISSING / STALE | <test name> | <incident id> |
| Integration (DB / queue / external) | EXISTS / MISSING / STALE | <test name> | <dependency> |
| Contract (API / event / schema) | EXISTS / MISSING / STALE | <test name> | <surface> |

## Gaps
<Concrete MISSING items, ranked by risk. Each gap is a candidate task — do not paper over it.>

## How To Run
```bash
<the actual command(s) for this area, e.g. go test ./controller/...>
```

## Evidence
```
Test files:   <paths to the test files backing the EXISTS rows>
Workflow:     <flow these tests cover>
Confidence:   HIGH | MEDIUM | LOW
```

<!-- MANAGED_END -->

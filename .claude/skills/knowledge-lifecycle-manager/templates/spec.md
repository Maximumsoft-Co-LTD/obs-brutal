---
source_commit: <sha>
generated_at: <YYYY-MM-DD>
generator: knowledge-lifecycle-manager
confidence: HIGH | MEDIUM | LOW
status: CURRENT | STALE | REVIEW | DEPRECATED
---

# Spec: <name>

> Sourced from workflows + business rules + testing knowledge — **not** generated directly from
> source code. Captures *intended* behavior so drift from *actual* (code) stays detectable.
> Code is a validation oracle only.

<!-- MANAGED_START -->

## Purpose
<What this component/endpoint is for.>

## Scope
<What is in scope and explicitly out of scope.>

## Inputs
| Input | Type | Required | Notes |
|-------|------|----------|-------|
| <field> | <type> | yes/no | <validation, source> |

## Outputs
| Output | Type | When | Notes |
|--------|------|------|-------|
| <field / status> | <type> | <condition> | <meaning> |

## Rules
- <BR-id reference> — <the behavior it imposes here>

## Constraints
<Performance, ordering, idempotency, timeout, rate-limit, security constraints.>

## Edge Cases
- <case> → <expected behavior> (Evidence / REQUIRES_HUMAN_REVIEW)

## Risks
<Known failure modes; cite risk-map entries.>

## Required Tests
- <test that must exist to consider this spec covered; mark MISSING if absent>

## Evidence
```
Workflow:      <backing workflow>
Business Rule: <backing rule(s)>
Test:          <backing test(s)>
Code Symbol:   <validation oracle — confirms, does not generate>
Confidence:    HIGH | MEDIUM | LOW
```
> Any assertion above with no backing workflow or test → REQUIRES_HUMAN_REVIEW.

<!-- MANAGED_END -->

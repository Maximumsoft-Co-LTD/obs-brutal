---
source_commit: <sha>
generated_at: <YYYY-MM-DD>
generator: knowledge-lifecycle-manager
confidence: HIGH | MEDIUM | LOW
status: CURRENT | STALE | REVIEW | DEPRECATED
---

# Business Rule: <BR-id — short name>

<!-- MANAGED_START -->

## Description
<The rule, stated as an invariant the system enforces. Precise enough to test against.>

## Why It Exists
<The reason. Cite the incident, regulation, or design decision that motivated it. If the why
cannot be recovered from evidence, mark REQUIRES_HUMAN_REVIEW rather than inventing one.>

## Impact If Violated
<What goes wrong: money lost, duplicate credit, fraud accepted, data corrupted.>

## Related Workflow
<workflow doc(s) where this rule is enforced — `[[workflow-name]]` or path.>

## Verification Method
<How to confirm the rule holds: the test that asserts it, or the manual check + its limits.>

## Evidence
```
Workflow:     <enforcing workflow>
Code Symbol:  <pkg.Func implementing the check>
Test:         <test asserting the rule, or MISSING>
Incident:     <incident that motivated/exposed it, if any>
Confidence:   HIGH | MEDIUM | LOW
```

<!-- MANAGED_END -->

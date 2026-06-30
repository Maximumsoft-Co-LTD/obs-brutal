---
source_commit: 8eea896
generated_at: 2026-06-24
generator: knowledge-lifecycle-manager
confidence: HIGH
status: CURRENT
---

<!-- Illustrative example for the skill, grounded in hash-central's core dedup purpose. -->

# Business Rule: BR-DEDUP-01 — one slip is credited at most once

<!-- MANAGED_START -->

## Description
A given transaction slip must result in **at most one** recorded deposit. Before recording, the service
hashes the transaction inputs and rejects the request if a matching hash already exists in
`transaction_statement` within the dedup window.

## Why It Exists
Prevents the same deposit from being credited twice — the founding purpose of the service. Double-credit
is direct money loss. (Why is recoverable from the service's stated core purpose + the dedup code path,
so confidence is HIGH; no human review needed.)

## Impact If Violated
A replayed or re-submitted slip credits the customer twice → unrecoverable financial loss and reconciliation breakage.

## Related Workflow
`[[slip-verify]]` — dedup happens at step 4 (`checkHashOrtherStatement`).

## Verification Method
`checkHashOrtherStatement` queries `transaction_statement` for the computed hash. Note a known edge: the
query windows ±1 min but the in-loop compare re-checks minute-exact, so a cross-minute duplicate can slip
through; the GSB `second > 54` guard is the partial plug. This nuance is itself a `REVIEW`-worthy gap.

## Evidence
```
Workflow:     slip-verify
Code Symbol:  controller.checkHashOrtherStatement
Test:         *_hashlayout_test.go (hash computation); dedup-window edge: MISSING
Incident:     —
Confidence:   HIGH (rule); MEDIUM (cross-minute edge → REQUIRES_HUMAN_REVIEW)
```

<!-- MANAGED_END -->

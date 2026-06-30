---
source_commit: 8eea896
generated_at: 2026-06-24
generator: knowledge-lifecycle-manager
confidence: MEDIUM
status: REVIEW
---

<!-- Illustrative example. Sourced from the slip-verify workflow + BR-DEDUP-01 + tests — NOT from code. -->

# Spec: POST /api/v2/slip-verify

> Sourced from `[[slip-verify]]` + `BR-DEDUP-01` + slip_verify tests. Captures intended behavior;
> code is a validation oracle only.

<!-- MANAGED_START -->

## Purpose
Verify a submitted slip and return its verification outcome, guaranteeing dedup.

## Scope
In: QR decode, bank verification (KTB direct + slip2go fallback), dedup, name match, response caching.
Out: crediting the customer (downstream office-api — invisible to this service).

## Inputs
| Input | Type | Required | Notes |
|-------|------|----------|-------|
| slip image (url or base64) | string | yes | hashed to `qr_decode:cache:` key |
| destination bank account | object | yes | resolves registered name for match |

## Outputs
| Output | Type | When | Notes |
|--------|------|------|-------|
| verified result | 200 | success | full success cached 5h |
| failure | 400 | verify/name/decode fail | error_type in `monitor_slipverify` |
| duplicate | 500 (observed) | hash already seen | maps to dedup reject in practice |

## Rules
- `BR-DEDUP-01` — reject if transaction hash already recorded.

## Constraints
- Per-attempt KTB timeout 10s; single orchestration attempt post-0015 (no 18× retry storm).
- slip2go fallback enabled on G2E only.

## Edge Cases
- Cross-minute duplicate → may evade dedup (REQUIRES_HUMAN_REVIEW — see BR-DEDUP-01).
- slip2go pre-masks BankNumber → breaks `processHash` on v2 path (F0019).

## Risks
KTB 403 anti-abuse storm (discrete, not chronic); see risk-map + incident-example.

## Required Tests
- KTB success + 403-fallback (EXISTS: slip_verify_ktb_test.go).
- Dedup cross-minute boundary (MISSING).

## Evidence
```
Workflow:      slip-verify
Business Rule: BR-DEDUP-01
Test:          slip_verify_ktb_test.go (EXISTS); dedup-edge (MISSING)
Code Symbol:   controller.verifyByBankAccount (oracle)
Confidence:    MEDIUM  → status REVIEW (dedup edge unconfirmed)
```
> The 500-for-duplicate mapping is inferred from observed behavior, not an explicit contract → flagged REVIEW.

<!-- MANAGED_END -->

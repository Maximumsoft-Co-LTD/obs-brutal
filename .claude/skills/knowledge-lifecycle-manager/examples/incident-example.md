---
source_commit: 7fe85ba
generated_at: 2026-06-24
generator: knowledge-lifecycle-manager
confidence: HIGH
status: CURRENT
---

<!-- Illustrative example, grounded in a real hash-central regression. In a live run this would live
     under the repo's existing convention (.workflow/<run-id>/ or FOLLOWUPS.md), not examples/. -->

# Incident: g2e-storm-break-continue-regression (2026-05-25 17:17)

<!-- MANAGED_START -->

## Problem
A KTB 403 anti-abuse storm hit the G2E tenant: slip-verify retried KTB aggressively instead of switching
device / failing fast, amplifying load on the rate-limited KTB endpoint.

## Symptoms
Sustained `errKtbAuthExpired403` / `[KTB-TXN-AUTH-REJECTED]` from ~17:17; near-100% slip-verify failure on
G2E during the window; KTB 403 rate spiking far above the ~6–8% baseline.

## Root Cause
Commit `7fe85ba` (branch `maan/fix/gen-device-by-req`) flipped v1.4.74's `break` to `continue` on the
`errKtbAuthExpired403` path. The `continue` kept the retry loop spinning on a credential KTB had already
rejected, instead of breaking out to fail over.

## Impact
G2E slip-verify effectively down for the storm window; multiplied 403 pressure on KTB (anti-abuse feedback
loop). No double-credit (dedup intact), so financial impact was availability, not money.

## Fix
Restore `break` on `errKtbAuthExpired403` so a 403 exits the retry loop immediately (consistent with the
0014 "403 retires device immediately" contract). Rollback path: this is code, not an env switch.

## Regression Test
A test asserting that a 403 from `fetchTransactionDetails` does **not** re-enter the same-credential retry
loop (counts KTB calls per request == 1 for the 403 case). Status at incident time: MISSING — this gap is
itself the lesson.

## Lessons Learned
`break` vs `continue` on the auth-expired path is a load-bearing distinction; the retry-budget math
(`worst_case = slip2goKTBMaxAttempts × outerMax × ktbMaxAttempts`) must stay bounded. Reinforces the A016
rule: KTB pool / slip-verify changes ship with the 3 mandated test files, including rate-limit/403 handling.

## Evidence
```
Code Symbol:  controller.fetchTransactionDetails (retry loop), errKtbAuthExpired403
Commit / Run: 7fe85ba (regression), v1.4.74 (correct prior behavior)
Test:         MISSING (regression test for 403 break)
Workflow:     slip-verify
Confidence:   HIGH
```

<!-- MANAGED_END -->

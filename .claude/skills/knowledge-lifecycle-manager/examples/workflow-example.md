---
source_commit: 8eea896
generated_at: 2026-06-24
generator: knowledge-lifecycle-manager
confidence: HIGH
status: CURRENT
---

<!-- Illustrative example for the skill. Grounded in hash-central's real slip-verify path so the
     shape and evidence style are concrete; symbols/paths are real, exact line numbers omitted by design. -->

# Workflow: slip-verify

<!-- MANAGED_START -->

## Purpose
Verify a bank transfer slip submitted by a tenant and decide verified / duplicate / failed, so the same
deposit is never credited twice. This is the core dedup-by-hash purpose of the service.

## Trigger
HTTP `POST /api/slip-verify` (legacy) and `POST /api/v2/slip-verify` (v2). Both enter the shared
`verifyByBankAccount` orchestrator (`controller/slip_verify_hash.go`).

## Steps
1. Decode QR from the slip image — `checkImageQr` (cache: `qr_decode:cache:`, kill-switch `QR_DECODE_CACHE_DISABLED`).
2. Resolve the destination bank account; load registered name for the name-match check.
3. Verify with the bank — KTB branch calls `fetchAccessToken` then `fetchTransactionDetails`; on failure,
   fall back to slip2go once (G2E tenant only).
4. Dedup: hash transaction inputs, look up `transaction_statement` via `checkHashOrtherStatement`.
5. Return verified / duplicate (HTTP 500 in practice) / new; cache full successes (`slip_verify:cache:`, 5h TTL).

## Dependencies
- **Modules:** `controller` (slip_verify_hash, slip_verify_ktb), `service/verifySlip`, `service/qrclient`, `service/blackHash`
- **Services:** KTB direct API, slip2go (fallback, G2E only), `maan-qrdecoder` (QR decode)
- **External integrations:** KTB prelogin/grant + transaction-details; optional `KTB_PROXY_URL`
- **Events / queues:** none — synchronous HTTP
- **Data stores:** MongoDB `transaction_statement`, `bank_account_slip`; Redis caches above

## Failure Scenarios
| Failure | Detection | Behavior | Evidence |
|---------|-----------|----------|----------|
| KTB auth rejected | `[KTB-TXN-AUTH-REJECTED]`, `errKtbAuthExpired403` | slip2go fallback (G2E) else fail | `controller.fetchTransactionDetails` |
| QR not decodable | `qrclient.ErrQRNotFound` (422) | `upstream_decode` error_type | `service/qrclient` |
| Duplicate slip | hash hit in `transaction_statement` | reject (HTTP 500) | `checkHashOrtherStatement` |
| Dest name mismatch | name compare fails | `name_mismatch` (self-heal gated by `SLIP_NAME_AUTOHEAL_DISABLED`) | `controller/slip_name_heal.go` |

## Evidence
```
Workflow:     slip-verify (self)
Code Symbol:  controller.verifyByBankAccount, controller.fetchTransactionDetails
Test:         slip_verify_ktb_test.go (fakeKtbServer + stubRedis)
Incident:     g2e-storm-break-continue-regression (see incident-example)
Confidence:   HIGH
```

<!-- MANAGED_END -->

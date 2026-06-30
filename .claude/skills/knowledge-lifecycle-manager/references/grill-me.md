# Grill-Me Mode

Purpose: **prevent hallucination** by surfacing what cannot be confirmed, instead of guessing past it.
This is the human-in-the-loop valve for everything the evidence model and confidence gate flag.

## Triggers

Enter Grill-Me when you hit any of:

- Missing workflow — a path exists in code but no workflow can be derived with confidence.
- Missing business rule — behavior implies a rule, but its *why* / authority is unknown.
- Unknown dependency — a call crosses a boundary you cannot resolve.
- Unknown ownership — no signal for who owns a flow / decision.
- Conflicting evidence — code and docs (or two docs) disagree.
- Legacy logic unclear — the behavior is observable but its intent is not recoverable from code+tests.
- Missing validation evidence — a claim can be stated but not backed by a test or symbol.

## Rules

1. **Do not stop the run immediately.** A trigger marks the *artifact* (LOW / `REQUIRES_HUMAN_REVIEW`),
   it does not abort the job.
2. **Collect questions** as you go into a running list.
3. **Batch** them — at most **4 questions per batch**.
4. **Ask at phase boundaries**, not mid-phase. One batch when a phase completes, then continue.
5. **Fan-out caveat:** when discovery is parallelized across sub-agents, workers *cannot* prompt the user
   — they return their questions to the orchestrator, which dedupes and asks the batch.
6. **Headless runs:** if no human is available, the batch is written to the Open Questions report and the
   run completes what it can. Never hang waiting for an answer.

## Question format

Each question carries enough context to be answerable without re-investigation:

```
Question:        <the specific decision needed>
Why It Matters:  <what downstream doc/decision depends on it>
Evidence Found:  <what the code/tests/docs DO show, with citations>
Missing Evidence:<the specific gap>
Possible Impact: <what goes wrong if guessed instead of confirmed>
```

## Example

```
Question:        Is a slip with status 500 a duplicate rejection or an upstream failure?
Why It Matters:  Determines whether docs/workflows/slip-verify.md documents 500 as success-dedup
                 or as a failure path; affects the error taxonomy in docs/business-rules.
Evidence Found:  controller returns 500 on both the dedup branch and the KTB-timeout branch
                 (verifyByBankAccount, no distinct status). No test asserts the dedup case.
Missing Evidence: no test, no comment, no incident distinguishing the two.
Possible Impact: mislabeling 500 would propagate a wrong rule into specs + AI guardrails.
```

A good question is one the user can answer in one line. If you cannot frame *Evidence Found* and
*Missing Evidence* concretely, you have not investigated enough to ask yet.

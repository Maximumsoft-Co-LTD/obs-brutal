# Evidence Model

Every workflow, business rule, spec, and testing artifact must carry **evidence** — a citation to
where the claim comes from. This is what makes "DO NOT GUESS" enforceable: a claim with no evidence
is not a claim, it is a `REQUIRES_HUMAN_REVIEW` or `UNKNOWN`.

## Two modes

The *primary* kind of evidence depends on which mode you are in. This matters because citing a derived
document as the source of another derived document is circular — during bootstrap the only ground truth
is the code and the tests.

### Bootstrap mode

Use when: first onboarding, or no knowledge base exists yet.

Primary (ground-truth) evidence, in order:
1. **Source code** (symbols, control flow)
2. **Tests** (assert observed behavior)
3. **Existing documentation** (corroborating, not authoritative on its own)

Workflows and specs are **being derived** in this mode — they are not yet authoritative and must not
be cited as primary evidence for each other. A workflow's evidence in bootstrap is code + tests.

### Maintenance mode

Use when: a knowledge base already exists and is being updated.

Primary evidence, in order:
1. **Workflow**
2. **Business rule**
3. **Test**
4. **Incident**
5. **Code symbol**

In maintenance mode, **code becomes validation evidence, not generation evidence** — you cross-check a
documented claim against the symbol to confirm it still holds, rather than re-deriving the doc from code.

## Evidence format

Attach an `Evidence:` block to each artifact. Prefer stable references over volatile ones.

```
Evidence:
  Workflow:    user-login
  Business Rule: auth-policy
  Test:        TestValidateUserStatus_Disabled
  Code Symbol: auth.ValidateUserStatus
  Confidence:  HIGH
```

### Reference preference (most → least stable)

1. **Code symbol** — `pkg.FuncName`, `Type.Method`. Survives line shifts; greppable. **Preferred anchor.**
2. **Workflow / rule / test / incident name** — semantic, stable across refactors.
3. **Line number** — *supplemental only, never primary*. If you need to point at a specific branch or
   magic value, write it as a pinned permalink, not a bare pointer:
   `auth.ValidateUserStatus (auth/status.go:120 @<commit-sha>)`. Without a commit it rots silently.

Avoid as primary evidence:
```
auth.go#L120-L145        # rots on the next edit; says nothing about what it proves
```

## Confidence and the gate

Evidence quality maps to the confidence gate (see `confidence-gate.md`):

| Evidence available | Confidence | Gate action |
|--------------------|-----------|-------------|
| Code + tests + docs agree | HIGH | Generate |
| Code present, tests/docs partial | MEDIUM | Generate + Review Flag |
| Insufficient / conflicting | LOW | Placeholder + `REQUIRES_HUMAN_REVIEW` + Open Question, continue |

## When evidence conflicts

Code and docs disagreeing is itself a finding — do **not** silently pick one:
- The code is the ground truth for *actual* behavior.
- The doc may encode *intended* behavior or hard-won operational knowledge.
- Record the conflict as an Open Question / Grill-Me item; mark the artifact `REVIEW`. Resolving it may
  belong to a human or to a code-change skill — not to this skill.

## What is NOT evidence

- A plausible-sounding inference with no citation.
- Another AI-generated doc cited as the source of a claim in bootstrap mode (circular).
- A commit message alone asserting behavior the code no longer has.

# Confidence Gate

Before generating any workflow, rule, spec, or testing artifact, assess the evidence behind it and pick
an action. The gate exists to keep low-evidence material from masquerading as fact, while still making
progress on a large repository.

## The gate

| Confidence | Evidence | Action |
|-----------|----------|--------|
| **HIGH** | Code + tests + docs agree | Generate normally. |
| **MEDIUM** | Code present; tests and/or docs partial or absent | Generate, but mark `status: REVIEW` and add a `Review Flag` noting what is unconfirmed. |
| **LOW** | Insufficient or conflicting evidence | Generate a **placeholder**, stamp `REQUIRES_HUMAN_REVIEW`, add an **Open Question**, and **continue** with the rest of the run. |

See `evidence-model.md` for what counts as evidence in bootstrap vs maintenance mode.

## LOW does not block the run

A LOW-confidence item halts *that one artifact*, never the whole job. Write the placeholder, record the
question, move on. This keeps a single ambiguous workflow from blocking an otherwise-complete knowledge base.

**Headless / autonomous runs:** there may be no human to answer at a phase boundary. Do not hang. Log the
Open Question, leave the `REQUIRES_HUMAN_REVIEW` marker, and finish everything you *can* do with evidence.
The Completion Report lists all such items so a human can resolve them later.

## Placeholder shape

A LOW placeholder is still a real, navigable doc — it just declares its own uncertainty:

```markdown
---
source_commit: <sha>
generated_at: <date>
generator: knowledge-lifecycle-manager
confidence: LOW
status: REVIEW
---

# Workflow: <name>   <!-- REQUIRES_HUMAN_REVIEW -->

> ⚠️ LOW confidence — insufficient evidence to document this safely.
> Open Question: <the specific thing that could not be confirmed>
> What IS known (with evidence): <the partial facts + citations>
> What is MISSING: <the gap>
```

Never fill the gap with a guess to make the doc "look complete." `UNKNOWN > INCORRECT`.

## Relationship to Grill-Me

LOW confidence is the most common trigger for a Grill-Me question, but it does not *stop and ask
immediately* — questions are collected and batched at phase boundaries (`grill-me.md`).

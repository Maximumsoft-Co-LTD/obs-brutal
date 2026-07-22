# Epic: <title>

**ID**: NNNN-type-slug
**Date**: YYYY-MM-DD
**Status**: epic
**Triggered by**: `Ship as: staged` + ≥ 2 independently-shippable capabilities

## Problem *(required)*

One paragraph: the user-facing problem this epic solves + why it needs staged releases (not one big drop).

## Slices *(required)*

Each slice becomes its own `/dev` run with `Parent: <this-id>` and must be shippable on its own (a real user gets real value when only that slice lands). Epic slices ship across **separate runs**; priority *within* one run is its `spec.md > User Stories` (P1/P2/P3) — different axis, don't conflate.

1. **<slug>** — <one-liner> · Acceptance: <one observable behaviour> · Size: small | medium | large

## Recommended starting slice *(required)*

Which slice goes first + why (smallest risk / unblocks others / fastest user value).

## Child runs *(required)*

Appended as slices spawn:

- [ ] `NNNN-feat-<slice-slug>` — created YYYY-MM-DD

---

**Optional sections** — add when it applies:

- **Dependencies** — cross-slice ordering ("Slice 2 needs <thing> from slice 1 — hard dep"); omit when independent
- **Out of epic** — what's NOT covered across all slices (future epics / never-doing), when scope-creep is a real risk

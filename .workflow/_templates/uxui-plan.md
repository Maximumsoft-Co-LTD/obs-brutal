# UX/UI plan: <title>

**Spec**: [./spec.md](./spec.md)
**Plan**: [./plan.md](./plan.md)
**Status**: draft | approved

> **For humans** — the design before any UI: Scenes (screens/states) → wireframes → Scenarios (user journeys) → AC map. Read top to bottom; `S1`/`AC1` just cross-link.

## Scenes *(required)*

Every screen / view / distinct UI state. A scene is a coherent surface (page, modal, panel); a different state of the same surface is a *state* (States column), not a new row.

| Scene | Purpose | Key elements | States (loading / empty / error / success) | Entry → Exit |
|-------|---------|--------------|---------------------------------------------|--------------|
| S1: <name> | <what the user does here> | <elements that carry the purpose> | <states + what each shows; `none` if static> | <how the user arrives → where each action leads> |

## ASCII wireframes *(required)*

Low-fi structure only (boxes, hierarchy, ordering, responsive changes). One per scene; desktop + mobile when layout changes across breakpoints.

### S1: <name> — desktop

```text
+--------------------------------------------------+
| Header / nav                                     |
+----------------------+---------------------------+
| Primary content      | Secondary panel / actions |
+----------------------+---------------------------+
```

## Scenarios *(required)*

Key journeys, each a walk across scenes. Cover the happy path AND the alternate/error flows the spec's boundary/error scenarios imply.

### SC1: <scenario name> (happy path)

- **Actor**: <who>
- **Precondition**: <what must be true>
- **Satisfies**: AC1, AC2

1. [S1] <user action> → <result / transition to S2>

## UX direction & components *(include unless a trivial single-state tweak)*

- **Direction / style**: <chosen style + why; cite spec References or the existing design system>
- **Information architecture / layout**: <nav model · primary grid · above the fold>
- **Key components**: <reuse existing (cite `path#anchor`); name net-new ones>
- **Interaction & feedback states**: <how loading / empty / error / success / disabled / focus read>
- **Responsive**: <breakpoints — ≥ mobile ≈375px + desktop; name every breakpoint the design changes at>
- **Accessibility**: <contrast · keyboard · focus order · labels/roles · WCAG level>

## AC ↔ scene mapping *(required)*

Every spec.md AC → the scene(s)/scenario(s) that satisfy it, and every scene/scenario back to an AC.

| AC | Scene(s) | Scenario(s) | Notes |
|----|----------|-------------|-------|
| AC1 | S1, S2 | SC1, SC2 | |
| AC1 (on error / boundary) | S2 (error state) | SC2 | |

---

**Hard rules** — states first-class · no orphan scenes/scenarios · every AC mapped · don't invent the unhappy path · reuse before invention · [NEEDS CLARIFICATION] for open requirement-level UX.

Full detail → **uxui.md > What goes in each section + Steps**.

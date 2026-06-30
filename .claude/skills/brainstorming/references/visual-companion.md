# Visual companion

A browser-based tool for showing mockups, diagrams, and side-by-side comparisons during the interview. It is a **tool**, not a mode — accepting it makes it available; it doesn't mean every question goes through the browser.

This reference only applies when (a) the upcoming questions are genuinely visual (UI, layout, diagram, design comparison) and (b) a browser automation surface is available (`mcp__claude-in-chrome__*` tools or a comparable browser MCP). If neither holds, skip this entire reference — text-only brainstorming is the default and there is no quality loss for non-visual work.

## When to offer

Offer the companion **once**, before the first visual question, *only when* visual content is genuinely coming up. Concrete triggers:

- The intent involves UI / UX / layout / page design / mockups / wireframes.
- You're about to ask the user to choose between two or more layouts, components, or visual styles.
- The architecture choice involves a diagram the user needs to see (not just read).

Do **not** offer when:

- The questions are conceptual (scope, AC, constraints, trade-offs without a visual answer).
- The work is backend-only / data-only / config-only.
- You haven't yet figured out whether there *are* visual questions — finish slot-walking first.

## The offer message — own message, nothing else attached

> "Some of what we're working on might be easier to explain with a visual. I can render mockups, diagrams, or side-by-side comparisons in the browser. This is opt-in and can be token-heavy — want to try it?"

**This MUST be its own message.** Do not combine with a clarifying question, a context summary, an apology, or anything else. The user needs to answer one question (yes/no) without juggling a second decision.

If you combine the offer with another question, the user usually answers the second question and the offer gets ignored — and now the visual context is poisoned for the rest of the conversation.

Wait for the response before continuing. If they decline, proceed text-only and never re-offer for this brainstorm.

## Per-question decision (after the user accepts)

Even after a yes, decide **per question** whether the answer is browser-better or terminal-better.

**The test:** would the user understand this better by **seeing** it than by **reading** it?

| Use the browser | Use the terminal |
|-----------------|------------------|
| Mockups, wireframes, side-by-side layouts | Requirements / scope / AC questions |
| Architecture diagrams the user needs to interact with | Yes/no decisions |
| Visual style comparisons | Trade-off lists |
| "Does this look right?" check | Free-text answers (e.g., Reproduction) |
| Color / typography / spacing choices | "What is the goal?" |

**A UI topic is not automatically a visual question.** Examples:

- "What does 'minimal' mean in this UI?" — conceptual → terminal.
- "Which of these two minimal layouts looks right?" — visual → browser.
- "Should the export button be primary or secondary?" — conceptual (it's a hierarchy decision) → terminal, *unless* you've already mocked the page and can show both states.
- "Where should the export button go?" — visual if there's a page layout to show → browser.

## Anti-patterns

- **Combining the offer with a clarifying question.** Always its own message.
- **Re-offering after a decline.** One offer, one answer, move on.
- **Running every question through the browser once accepted.** That's "mode," not "tool." Per-question decision still applies.
- **Offering when no visual questions are coming.** Costs a turn and confuses the user. Slot-walk first.
- **Offering before the browser tools are loaded.** If the schemas haven't been loaded via `ToolSearch`, you can't actually use them — load first, then offer.
- **Generating elaborate mockups for one yes/no question.** The cost should match the question. A 3-option layout comparison: mockups make sense. "Should the button be blue or green?" — a 4-color swatch in text is fine.

## Mini protocol (if accepted)

1. Load the browser tools you'll actually need (`ToolSearch` with `select:mcp__claude-in-chrome__<tool>,...`).
2. Get tab context (`mcp__claude-in-chrome__tabs_context_mcp`) before creating a tab.
3. Render the visual (mockup / diagram / comparison).
4. Ask the question in text — the visual is the **prop**, the question is still a normal `AskUserQuestion` or short prompt.
5. Read the answer, decide whether the next question is also visual; if not, switch back to terminal-only for that question.

If anything in this protocol fails (tools won't load, browser unreachable, render errors twice), abandon the companion and fall back to text. Don't keep retrying — the brainstorm is the goal, not the visual.

## Why this reference is here

The companion is genuinely useful for UI / layout / diagram work, but it has three failure modes that show up consistently:

1. The offer combined with a question — user only answers one of them.
2. "Mode-creep" after acceptance — every question goes through the browser, even conceptual ones.
3. Offered when no visual questions actually arise — wasted turn.

This reference exists to prevent those three. If you can keep them in mind without re-reading, you don't need this file.

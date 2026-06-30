# Rule: Coding discipline by default

For every task where you are about to produce or edit code — implement a feature, fix a bug, refactor, change behavior, "clean something up" — invoke the `coding-discipline` skill **before** writing the first line. This is the behavioral conduct layer that wraps all the domain fundamentals: it governs *how you show up to a code task*, not *what you build*.

This rule is the always-on pointer. The 4 principles (think before coding → simplicity first → surgical changes → goal-driven execution), the pre-flight checklist, and the routing guidance to the skills that own each concern all live in the skill:

- `.claude/skills/coding-discipline/SKILL.md`

**Why:** The most expensive AI-coding failures aren't knowledge gaps — the model already knows better. They are *conduct* gaps, and they recur: acting on a silent assumption and running 200 lines down the wrong road; bloating a 100-line job into 1000 with speculative abstraction; "improving" adjacent code the change never needed and breaking something orthogonal; shipping with no checkable definition of done so the user becomes the test harness. (Adapted from [Andrej Karpathy's note on LLM coding pitfalls](https://x.com/karpathy/status/2015883857489522876), via the MIT-licensed `multica-ai/andrej-karpathy-skills`.) Catching these as a stance check costs seconds; missing them costs the rewrite, the buried diff, and the lost trust.

**How to apply:** At the start of any code task, load the `coding-discipline` skill and run the 4-question pre-flight (assumptions stated and ambiguity named → minimum non-speculative code → every changed line traces to the request → concrete definition of done). The skill itself lists when to skip (pure config edits, one-line shell, throwaway scripts) — defer to it rather than re-deciding here.

**Relation to other skills:** Coding-discipline is a thin behavioral **wrapper**, deliberately not a competitor — it *routes* to the skills that own each concern instead of duplicating them. "Think before coding" escalates the full ambiguity/scope conversation to [[brainstorming]]; "simplicity first" is write-time intent whose mechanics live in [[programming-fundamentals]] (complexity, illegal states, pure core) and whose post-hoc cleanup is `/simplify`; "surgical changes" is the write-time cousin of [[git-workflow]]'s atomic-commit discipline; "goal-driven execution" sets the verification stance that [[debug-fundamentals]] and the `qa` agent then execute. Run order: apply this skill **first** as the conduct check on any code task, then the layer-appropriate construction or debug skill (`ddd-strategic` → `programming-fundamentals` → `database-fundamentals` → `hexagonal-backend` → `architecture-fundamentals` → `queue-fundamentals`, or `debug-fundamentals` first for a bug). It does not replace any of them and must not re-teach their content.

This rule also pairs with the `/dev` workflow: principle 1 (think first) is the always-on micro-version of the Phase 1 brainstorming/spec step, and principle 4 (goal-driven) mirrors the workflow's verifiable acceptance criteria and the `qa` test-mapping in Phase 2.

**Status:** Active. Applies to all code-producing work in this project and any project that adopts this foundation.

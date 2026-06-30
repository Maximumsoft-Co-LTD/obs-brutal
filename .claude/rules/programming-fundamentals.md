# Rule: Programming fundamentals by default

For every code task with real logic — writing a function, designing a module, modeling data, fixing a non-trivial bug, refactoring, or reviewing code — invoke the `programming-fundamentals` skill **before** writing or substantially changing code.

This rule is the always-on pointer. The 7 principles, pre-flight checklist, and deep-dive guides on naming, error handling, complexity, and testing live in the skill:

- `.claude/skills/programming-fundamentals/SKILL.md`

**Why:** Most defects, hard-to-debug systems, and painful rewrites trace back to the same missed fundamentals — sloppy data shapes, illegal states left representable, functions doing too much, side effects mixed with logic, swallowed errors, accidentally quadratic loops, and bad names. Catching these at *write* time costs seconds; catching them in production costs days.

**How to apply:** At the start of any meaningful code task, load the `programming-fundamentals` skill and run the 7-principle pre-flight (data shape → illegal states → one-thing functions → pure core → error handling → complexity → read first). Apply the relevant reference file when the work is concentrated in one area (e.g., heavy renaming → `references/naming.md`). The skill lists when to skip (one-line shell, throwaway scripts, pure config edits) — defer to it rather than re-deciding here.

**Relation to other skills:** Programming fundamentals are the code-level layer. They compose with [[ddd-strategic]] (decides *where* the model lives and *what language* it speaks — runs **before** this), [[hexagonal-backend]] (one service's internal layering), [[architecture-fundamentals]] (system-level structure — runs **after** this), and [[simplify]] (post-hoc review) rather than competing. Canonical construction run order: `ddd-strategic → programming-fundamentals → database-fundamentals → hexagonal-backend → architecture-fundamentals → queue-fundamentals` (the master order in CLAUDE.md). Within it, run this *before* storage and architecture — get the fundamentals right first. ("Run this first" is scoped to that chain; the always-on [[coding-discipline]] conduct check still wraps it, and for a bug [[debug-fundamentals]] runs first to find the cause.)

**Status:** Active. Applies to all code work in this project and any project that adopts this foundation.

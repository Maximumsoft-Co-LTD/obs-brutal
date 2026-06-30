# Rule: Database fundamentals by default

For every task that touches a database — designing a schema, writing a non-trivial query, adding an index, writing a migration, debugging slow queries, or modeling persistent data — invoke the `database-fundamentals` skill **before** writing schema, SQL, or migration code.

This rule is the always-on pointer. The 7 principles, pre-flight checklist, and deep-dive guides on indexing, transactions, query performance, and migrations live in the skill:

- `.claude/skills/database-fundamentals/SKILL.md`

**Why:** Most production database pain — slow pages, outages during deploys, mysteriously corrupt data, mounting tech-debt schemas — traces back to the same handful of missed fundamentals. Bad types and missing constraints let bad data through; missing or wrong indexes turn linear queries quadratic; long transactions wedge the database under load; sloppy migrations cause downtime. Catching these at *design* or *write* time costs minutes; catching them in production costs hours of downtime, weekend pages, or data-loss postmortems.

**How to apply:** At the start of any database-touching task, load the `database-fundamentals` skill and run the 7-principle pre-flight (model the data → push constraints into the schema → indexes match queries → read the plan → fetch in sets, not loops → keep transactions short → migrations are expand → backfill → contract). Apply the relevant reference file when the work is concentrated in one area (e.g., a slow query → `references/query-performance.md`; a schema change on a live table → `references/migrations.md`). The skill lists when to skip (throwaway scripts, one-off ad-hoc queries, pure config edits) — defer to it rather than re-deciding here.

**Relation to other skills:** Database fundamentals compose with [[programming-fundamentals]] (the layer below) and [[hexagonal-backend]] (which puts the database behind an adapter port). They are not competing — fundamentals are *what* you put behind the port; hexagonal is *how* you isolate it. If multiple skills apply, run `programming-fundamentals` first, then this skill, then `hexagonal-backend` for layering. Getting the schema and the access patterns right matters even more than the architecture wrapping them — a clean port over a broken schema still gives you a broken system.

**Status:** Active. Applies to all database work in this project and any project that adopts this foundation.

# Rule: Hexagonal backend by default

For every backend task with real domain logic — services, APIs, repositories, use cases, business logic, persistence, message handling — invoke the `hexagonal-backend` skill **before** designing or writing code.

This rule is the always-on pointer. The full layering rules, port/adapter patterns, dependency direction, testing strategy, folder structures (TypeScript and Go), and common pitfalls live in the skill:

- `.claude/skills/hexagonal-backend/SKILL.md`

**Why:** Standing preference. Hexagonal / ports-and-adapters keeps the codebase resilient to requirement changes — swapping a DB, framework, transport, or external API touches only adapters, not core logic. Requirements change often; this architecture absorbs that change cheaply.

**How to apply:** At the start of any backend task, load the `hexagonal-backend` skill and follow its layering rules (domain / application / infrastructure with dependency pointing inward). Don't write controllers or DB code before defining ports. The skill itself lists when to skip (throwaway scripts, trivial CRUD with no real domain, thin BFFs) — defer to it rather than re-deciding here.

**Status:** Active. Applies to all backend work in this project and any project that adopts this foundation.

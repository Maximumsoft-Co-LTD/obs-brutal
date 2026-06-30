# Team agents — manifest

Local fanout workers embedded under `.claude/agents/` for use by the `/dev` fanout pattern (see `.claude/skills/fanout-team-agents/SKILL.md`). Some are foundation-native spec/plan workers; the review workers are local forks of upstream review agents. This file is the audit trail: source path per agent, fork date when applicable, version inferred at fork-time, and the drift-awareness rule.

## Naming convention

Every embedded review agent uses the filename prefix `team-<role>.md`, and the `name:` YAML field is renamed in lock-step to match the filename slug (e.g., file `team-code-reviewer.md` ↔ `name: team-code-reviewer`). Three reasons:

1. **No shadowing of the 5 `/dev` workers** (`pm`, `lead`, `engineer`, `qa`, `retro`). The `team-` prefix gives visual separation so a reader of `lead.md`'s review mode cannot mistake `team-code-reviewer` for `lead` itself — `lead` remains the synthesiser/owner of `review.md`; the embedded `code-reviewer` is one of the fanned-out workers.
2. **Filename ↔ `name:` lock-step is load-bearing.** Claude Code's sub-agent spawn surface uses the `name:` YAML field; if the filename and `name:` drift the agent cannot be spawned. The rename verify-clause in plan steps 4–9 enforces this.
3. **Flat directory layout** under `.claude/agents/`. Subfolder support is undocumented in Claude Code — every observed marketplace plugin keeps agents flat. The `team-` prefix achieves visual grouping without depending on subfolder discovery.

## Foundation-native workers

- team-codebase-explorer — read-only worker for spec/plan codebase exploration.
- team-best-practice-researcher — research worker for spec/plan best-practice probes.

These are foundation-native because the existing review-agent forks are diff-oriented; spec/plan needs pre-diff exploration and research workers with read-only output contracts.

## Fork sources

Forked: **2026-05-21**. Source plugin: **`pr-review-toolkit`** at `~/.claude/plugins/marketplaces/claude-plugins-official/plugins/pr-review-toolkit/agents/`. Version inferred from the marketplace cache at fork-time (not pinned to a release tag — `pr-review-toolkit` did not carry a version manifest in the cache snapshot).

- team-code-reviewer — `pr-review-toolkit/agents/code-reviewer.md` → `.claude/agents/team-code-reviewer.md`
- team-code-simplifier — `pr-review-toolkit/agents/code-simplifier.md` → `.claude/agents/team-code-simplifier.md`
- team-comment-analyzer — `pr-review-toolkit/agents/comment-analyzer.md` → `.claude/agents/team-comment-analyzer.md`
- team-pr-test-analyzer — `pr-review-toolkit/agents/pr-test-analyzer.md` → `.claude/agents/team-pr-test-analyzer.md`
- team-silent-failure-hunter — `pr-review-toolkit/agents/silent-failure-hunter.md` → `.claude/agents/team-silent-failure-hunter.md`
- team-type-design-analyzer — `pr-review-toolkit/agents/type-design-analyzer.md` → `.claude/agents/team-type-design-analyzer.md`
- team-dispatching-skill-source — `~/.claude/plugins/cache/claude-plugins-official/superpowers/5.0.7/skills/dispatching-parallel-agents/SKILL.md` (pattern source for `.claude/skills/fanout-team-agents/SKILL.md` only; no agent fork from `superpowers` ships — Open question E of spec 0002 resolved in favor of `pr-review-toolkit`'s `code-reviewer` and dropped the `superpowers` variant to avoid two competing reviewers)

Fork date: 2026-05-21

## Drift awareness

Upstream parity is **not** enforced for forked agents. Drift is expected — the local forks are owned by this repo and will pick up local conventions (e.g., references to this repo's `CLAUDE.md` rules, this repo's logging functions, this repo's test framework). Foundation-native workers (`team-codebase-explorer`, `team-best-practice-researcher`) do not have upstream parity obligations. The rules:

- Any change to a `team-*` agent file must update the corresponding `Fork source:` block (top of the file, under the YAML) — at minimum, set a new `forked:` date or add a `local-edit:` line citing what changed.
- An audit pass against upstream is a follow-up, not a recurring obligation. The audit diffs each local file against the source path above and decides per-finding whether to keep the local divergence or pull upstream.
- If upstream `pr-review-toolkit` ships a new version with a structural change (new YAML fields, new output format), the audit is the trigger to re-evaluate the forks; this file's fork-date stamp is the reference point.

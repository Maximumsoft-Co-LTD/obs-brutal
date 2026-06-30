# Orchestrator (main-agent role)

> **This file is not a sub-agent.** It lives at `.claude/orchestrator.md` (not under `.claude/agents/`) on purpose: there is no `orchestrator` agent — Claude Code sub-agents cannot use `Agent` (no nested spawns) or `AskUserQuestion` (sub-agents can't talk to the user), so orchestration must run in the **main agent**. The `/dev` slash command reads this file and the main agent follows it. Worker sub-agents you spawn from here are exactly: `pm`, `lead`, `engineer`, `qa`, `retro`; fanout worker sub-agents are the `team-*` agents. **Never** call `Agent(subagent_type="orchestrator")` — that name does not exist and the spawn will fail with `Agent type 'orchestrator' not found`.

You — the main agent reading this — are the Orchestrator for `/dev`. You drive the flow; sub-agents do the substantive file work; you handle every `Agent` spawn and every `AskUserQuestion`. The flow is **type-aware**: some phases run, some are skipped, and one (security review) is trigger-based. See `WORKFLOW.md > Type-aware phase matrix` for the truth table.

## On invocation

### Fresh run

1. Read `.workflow/INDEX.md` and `.workflow/FOLLOWUPS.md`. Consult `WORKFLOW.md` only for the specific section needed to choose phases, security triggers, or resolve an unclear workflow rule.
2. **Repo detection.** Run `find . -maxdepth 2 -name .git -type d 2>/dev/null` to discover git repos under the working directory.
   - `./.git` only (no subdirectory `.git` dirs): **single-repo** — `repo_root = $(pwd)`. No question needed.
   - Any subdirectory `.git` dirs found (whether or not the root itself also has `.git`): **control-plane** — ask via `AskUserQuestion` (one question, "Which repo does this run target?"); list each discovered path (strip the trailing `/.git`); if the root itself also has `.git`, include `$(pwd)` as the first option labelled `<dirname> (this repo)`; include Other for a custom path. `repo_root = <selected absolute path>`.
   - No `.git` found anywhere: **no-git** — `repo_root = null`; skip branch creation entirely.
3. Pick the next run ID: `NNNN-<type>-<kebab-slug>`. Type is one of `feat|fix|refactor|chore|docs|spike`. Propose branch name: `<type>/<kebab-slug>` (e.g. `feat/todolist-app`). Ask via `AskUserQuestion` — one batch of up to 2 questions:
   - **(If type is unclear)** "Run type?" — the 6 type options with one-line descriptions.
   - **(If `repo_root` is set)** "Branch name?" — first option is the proposed name (Recommended); Other for custom input.

   **As soon as the branch-name answer comes back, create and checkout the branch immediately — before creating the run folder (step 4) or any later step.** Skip this only when `repo_root` is null (no-git). Otherwise, in order:
   1. Check the base: run `git -C <repo_root> branch --show-current`. If the current branch is not `main` or `master` (or the repo's configured default), warn the user and ask via `AskUserQuestion` whether to checkout the default branch first (recommended) or branch from the current head; act on the answer.
   2. Create + checkout: run `git -C <repo_root> checkout -b <branch>`. If the branch already exists, run `git -C <repo_root> checkout <branch>` instead and note `branch_existed=true` in state.
   3. Confirm to the user which branch is now checked out, then continue to step 4. Do not proceed to step 4 until the checkout has actually run.
4. Create the run folder `.workflow/<id>/`.
5. Copy `.workflow/_templates/state.json` to `.workflow/<id>/state.json`. Fill `id`, `type`, `repo_root`, `branch`, `phase=phase-1-requirements`, `step=interview`, `last_updated=<ISO timestamp>`.
6. Append a row to `.workflow/INDEX.md`: status = `spec`, started = today, finished = `—`.

### Resume (`/dev --resume <id>`)

1. Read `.workflow/<id>/state.json`.
2. If `repo_root` is set: run `git -C <repo_root> checkout <branch>`. If this fails for any reason (dirty tree, branch missing, detached HEAD, or any git error) — **stop immediately and surface the error to the user via `AskUserQuestion` before proceeding**. Never continue a resume on an incorrect or unverified branch.
3. Print one sentence: "Resuming `<id>` at phase=<phase>, step=<step>, cycles=review:<n>/test:<n>, repo=<repo_root>, branch=<branch>."
4. Jump to the matching step below. Don't replay completed steps. If `state.json` is missing or malformed, ask the user (`AskUserQuestion`) whether to start fresh.

## State discipline

After **every** step (success, deviation, or cycle-bump), update `.workflow/<id>/state.json`:
- `phase` and `step` reflect the *just-completed* step.
- `next_step` names what runs next per the type matrix (skipped steps included as `"skipped:<reason>"`).
- `cycles.review` / `cycles.test` bump only on actual cycle increments.
- `last_updated` is a fresh ISO timestamp.
- `last_agent` is the agent that just returned (`main` when you did the work yourself, e.g. the interview).

Without state writes, resume is broken. Don't skip them even when the next step is "obvious".

This rule is now hook-enforced: `.claude/hooks/dev-state-mark.sh` (PostToolUse on `Agent`) touches `.workflow/<id>/.last_worker_return` whenever a worker (`pm | lead | engineer | qa | retro`) returns, and `.claude/hooks/dev-agent-guard.sh` (PreToolUse on `Agent`, case 3) blocks the *next* worker spawn until `state.json` mtime is newer than that marker. If you see `BLOCKED by /dev guard: .workflow/<id>/state.json was not updated after the last worker returned`, write `state.json` (Write/Edit) with the just-completed step before retrying — that *is* the missing step.

### Between-step efficiency

The slowest part of a `/dev` run is usually *you* — the main-agent turn between two worker spawns. Keep those turns lean so the next spawn fires sooner and stays cache-warm:

- **`state.json` + the returning worker's summary are your working set.** Don't re-read `spec.md`, `plan.md`, or artifacts already summarised in context. Re-open a file only when a step explicitly requires it (e.g. the post-spec "Spec check" opens `spec.md`; the gate reads `plan.md`).
- **Short turns keep spawns fast and cheap.** The prompt cache has a ~5-minute TTL; a long main-agent turn between spawns lets the shared prefix go cold, so the next worker reprocesses it uncached — slower *and* more expensive. Decide, write `state.json`, spawn — don't narrate or re-derive.
- **Fanout goes out in one message.** When 2+ independent probes are warranted, dispatch them all in a single turn per `## Fanout dispatch` — sequential `Agent` calls across turns are not parallel and multiply wall-clock.
- **Pass repo context to every sub-agent.** Include `repo_root` and `branch` from `state.json` in every sub-agent prompt. Sub-agents that run git or file operations must scope them to `repo_root` (e.g. `git -C <repo_root> …` or `cd <repo_root>` before any git command).

## Phase 1 — Requirements

6. **Interview (you run it).** You — the main agent — run the spec interview. Sub-agents can't call `AskUserQuestion`, so this step lives here, not in `pm`.
    1. **Use the `brainstorming` skill sparingly — it is on the spec critical path.** The full `brainstorming/SKILL.md` + its references are ≈ 47 KB; reading them is a chain of inference round-trips that slows every spec. **Default: run the interview from the slot discipline below + the always-on summary, without loading the skill body.** Reach for it only when scope is genuinely ambiguous, open-ended, or oversized (true decomposition needed), and even then prefer a single targeted reference (e.g. `brainstorming/references/interview-tactics.md`) over the whole skill. For narrow, concrete changes — the common case — do not load it at all.
    2. Read `.workflow/_templates/spec.md` and `.workflow/FOLLOWUPS.md`. Skim the `Open` table — if any item looks like it could be in scope for this intent, fold it into the questions.
    3. **Spec-prep fanout (condition-based).** This is not an every-run step. Dispatch focused workers before asking the interview batch only when doing so will reduce guessing: existing modules/integration points are named, the work touches APIs/security-sensitive paths, the product/domain terms are unfamiliar, or there are 2+ independent research questions.
       - `team-codebase-explorer` per codebase area / integration point to discover current behaviour, invariants, and likely constraints.
       - `team-best-practice-researcher` per external API / framework / security / UX / architecture practice question to gather current best-practice constraints.
       Dispatch all probes in one message. If a `team-*` spawn fails with `Agent type ... not found`, use the inline fallback in `## Fanout dispatch`. Save the labelled findings and `Dispatched-as:` map for the `pm` prompt. Skip this fanout for XS pure-greenfield work with no unfamiliar domain/API choices.
    4. Read `.claude/agents/pm.md > Required slots` for the full slot list. Pick the 3–4 slots the intent left UNSPECIFIED, using the prep findings to avoid asking questions the codebase already answers. **Never** assume defaults for slots you didn't ask about. For any slot the **repo** answered (not the user) — stack, integration point, convention — record it in an `Assumptions (inferred)` list; you will surface it at the gate for a one-line veto (a wrong inference passes every consistency scan). For feat/fix that ships a runtime path, the **NFR detection** question (binary: is there a measurable perf/security/a11y target?) is mandatory in the batch budget — on `yes` the target is written as an AC (verify = its `measured:` clause), not a standalone section that orphans it. For each consequential *behavioural* AC, also detect the **error/boundary** behaviour (what happens for bad input / a hit limit / an unauthorized caller) — `none — <default>` is a valid recorded answer, silence is not; this is the EARS IF/THEN clause that stops the implementer guessing the unhappy path. (An NFR-class AC — a measurable target with a `measured:` clause — is exempt: that clause is its verify.)
    5. Call `AskUserQuestion` — **one batch of 3–4 questions by default**. Prefer multi-choice options with one-line descriptions. For `fix` runs, the reproduction question is free-text; do not invent steps. Never skip this — even a one-line intent like "create todolist" needs the batch. Capture a concrete `input → expected output` example AND the `on error / at boundary:` behaviour for each consequential *behavioural* AC (Specification by Example + the EARS unhappy-path clause); an NFR-class AC instead carries just its `measured:` clause. **Bounded dig loop:** for high-ambiguity work (Type still unclear after batch 1, >4 consequential slots open, or a batch-1 answer opened a new unknown the options didn't cover) you MAY run a 2nd — at most 3rd — batch that digs *narrower* into what the prior answer revealed (hard cap 3; never re-open cold slots; each batch narrower than the last). If the picture is still open after 3 batches, stop and mark `[NEEDS CLARIFICATION]` rather than guessing. Full discipline: `brainstorming > references/interview-tactics.md`.
    6. Save the answers, the `Assumptions (inferred)` list, captured AC examples, any folded-in follow-up IDs, and any fanout findings for the `pm` spawn.
7. **Spec.** Spawn `pm` (mode = "write spec from answers"). Pass the run id, the type, the user's intent, the full Q&A from step 6, the list of carried-over `FOLLOWUPS.md` IDs the user confirmed are in scope, and any spec-prep fanout findings + `Dispatched-as:` map. `pm` writes `.workflow/<id>/spec.md` from the template + answers and returns the spec path + 3-bullet summary. If `pm` returns `FANOUT_REQUESTED: research:<question-list>`, follow `## Fanout dispatch` below — dispatch `team-codebase-explorer` for `codebase-*` questions and `team-best-practice-researcher` for `best-practice-*` questions, then re-spawn `pm` with the worker findings appended to the interview Q&A. Update INDEX status → `planned`. Update state: `step=spec, next_step=plan`.
   - **Spec check.** Open `spec.md`. Confirm: `Type` is set; minimum floor (Goal + AC) is present; every section that appears has its template-comment trigger firing (no "just in case"); for `fix`, `Reproduction` has concrete steps; for `spike`, `Timebox` has a hard limit; any unresolved slot is marked with an inline `[NEEDS CLARIFICATION: <who> — <what>]` (see `pm.md > Inline ambiguity` for syntax). Re-spawn `pm` only if a slot has a real answer that didn't make it in.
8. **Plan.** **Plan-prep fanout first (push-based, speed-oriented).** Before spawning `lead`: when `repo_root` is set AND `spec.md > Constraints > Integration points` names **≥ 2** points in existing code (or one large/unfamiliar one), dispatch **one `team-codebase-explorer` (haiku) per integration point in a single message** (parallel — same rule as `## Fanout dispatch`) to map current state for that point — entry point → data/control flow → callers/blast-radius → invariants, each citing `path#anchor`. This offloads the slow, serial LSP-walk off `lead`'s critical path onto parallel haiku workers — a win **when there are genuinely several points to map in parallel**, since each explorer pays its own LSP cold-start and context load (≈ N× the warm-up `lead` would pay once), so for a borderline 2-point case the round-trip + N× cold-start can erode the gain. The `≥ 2` threshold is therefore a **tuning knob — validate against real wall-clock** and raise it if small-N prep doesn't beat letting `lead` walk the points itself. **Skip `team-best-practice-researcher` here** unless `spec.md` flags an unfamiliar framework/API/security choice — web research is the slowest worker, opt-in only. **Skip the prep entirely** for XS/S, pure-greenfield, or a single simple integration point (the extra round-trip isn't worth it — let `lead` walk it). Then spawn `lead` in **plan mode** to write `plan.md` (or `epic.md` if scope check triggers), **appending the explorer findings + a `Dispatched-as:` map** so `lead` synthesises them into `Current state` instead of re-walking those points. Pass the `Type` so the plan applies the right rules (regression-test-first for `fix`, behavior-equivalence note for `refactor`, timeboxed exploration for `spike`). **Model override (speed, quality-preserving): spawn plan mode with a `model: sonnet` override by default** — plan mode is `plan-writing`-skill-guided (the skill prescribes sizing, section gating, the strict step format, and verify/AC discipline), so sonnet matches opus output here at roughly half the wall-clock. **Exception — keep opus:** when `spec.md` signals L-tier architectural complexity (cross-subsystem change, schema migration, public API / event-contract change, or any breaking change — the L triggers in `plan-writing > references/size-tiering.md`), omit the override so `lead`'s `opus` frontmatter stands and the hardest plans keep the stronger model. This is decidable from `spec.md > Constraints`/scope **before** the spawn. Review (step 11) and security (step 12) always keep opus. Update INDEX status → `planned` (or `epic`). State: `step=plan, next_step=gate`. If `lead` Mode A still returns `FANOUT_REQUESTED: plan:<point-list>` (the pull-based fallback for anything the push-prep above did not cover — typically best-practice research, or integration points that surfaced only mid-plan), follow `## Fanout dispatch` below — for each point, dispatch `team-codebase-explorer` for current-state mapping **(skip this worker for any point the plan-prep already mapped — re-dispatch only the residual `team-best-practice-researcher`)** and `team-best-practice-researcher` for relevant framework/API/testing/security best practices, then re-spawn `lead` for synthesis into `Current state`, `Research notes`, `Approach`, `Risks`, and `Steps`.
   - **Plan check.** Read `plan.md` (or `epic.md` if the scope check produced an epic). Confirm: a `Steps` section exists with at least one step (for an epic, at least one slice); no `[NEEDS CLARIFICATION]` markers remain. If either check fails, re-spawn `lead` (plan mode) **once** with the issue noted; if it still fails, escalate to the user via `AskUserQuestion`.
9. **Gate** — Build the type-aware run plan from the matrix in `WORKFLOW.md`. Decide whether to open a PR on ship (the spec has the user's preference; if it's still blank, ask once with `AskUserQuestion`). Tentatively mark whether security review will fire (rule of thumb: if any planned file path or diff hint matches the sensitive-paths list, plan to fire; the real decision happens after implement). Print a tight summary:
    - Spec goal + Type + `Ship as`
    - **The acceptance criteria as the contract** — list every AC (with its `e.g.:` example and its `on error / at boundary:` behaviour, if any, plus any `measured:` target now carried as an AC) and frame it explicitly: *"I'll treat this work as done when each of these is true — confirm each line, or correct the ones that aren't what you meant."* This is a per-line confirmation, not a wholesale glance: the AC are the only user-validated link in the `spec → plan → code` chain, so they earn an explicit sign-off.
    - **`Assumptions (inferred from repo — correct any that are wrong)`** — the inferred-answer list from step 6 (stack, integration points, conventions the repo answered rather than the user). Empty list → omit this bullet.
    - Constraints / stack from `spec.md`
    - Plan outline (or epic slices) + risks + rollback summary
    - The type-aware step list: `"Will run: 1, 2, 3, 4, 5, 7, 9, 10. Skipping: 6 (no sensitive paths planned), 8 (type=fix, docs not in scope). Open PR on ship: yes."`
    - Any open follow-ups the interview flagged as candidates.
    Ask the user via `AskUserQuestion`: `approve` | `revise <notes>` | (if epic) `swap <n>`. A correction to any AC line or any inferred assumption is a `revise`.
   - `revise` → loop to step 6 (re-interview only if the notes affect requirements; otherwise edit `spec.md` inline — resolve or add `[NEEDS CLARIFICATION]` markers per `pm.md > Inline ambiguity` — and re-spawn `pm` for a spec patch), then re-spawn `lead` for a fresh plan. Before approval at this gate, confirm zero markers remain.
   - `swap` → ask `lead` (plan mode) to open the chosen slice as the active run.
   - `approve` → INDEX status → `approved`. State: `step=gate, next_step=implement`. Proceed.

## Phase 2 — Implementation (autonomous)

10. **Implement.** Spawn `engineer` in implement mode. Pass the `Type` and the spec's acceptance criteria. INDEX status → `building`. State: `step=implement`.
    - For `fix`, restate in the prompt: "Step 1 is writing the failing regression test from `spec.md > Reproduction`. Commit the failing test as its own commit before any production fix. Do not change the buggy code before the test fails on the existing code."
    - Engineer must tick each `spec.md > Acceptance criteria` checkbox or file a blocker. On return, confirm acceptance progression in `spec.md`; if the engineer left criteria unticked without a blocker note, re-spawn with one correction.
    - **Diff check** (skip when `repo_root` is null). Confirm the engineer actually produced work. For `spike`, confirm `.workflow/<id>/recommendations.md` exists instead. For `fix` (the engineer commits the test + fix during implement, leaving a clean tree), confirm HEAD advanced — `git -C <repo_root> log --oneline -2` shows this run's commits. For all other types, run `git -C <repo_root> status --porcelain` (catches staged, unstaged, **and untracked new files**, which `git diff` misses); if it lists no files, the engineer changed nothing — re-spawn with that note.
    - If `engineer` returns "needs user input" (e.g., unfamiliar files in `git status` during ship, destructive op confirmation), surface the question to the user with `AskUserQuestion` and re-spawn `engineer` with the answer.
    - If `engineer` returns `FANOUT_REQUESTED: implement:<phase-list>`, follow `## Fanout dispatch` below. Treat this shape as experimental — see the caveat in that section.
11. **Review.** Spawn `lead` in **review mode**. INDEX status → `review`. State: `step=review, cycles.review++`.
    - `lead` Mode B may return `FANOUT_REQUESTED: review` when the diff is large, crosses multiple modules, touches critical behaviour, changes types/contracts, alters tests, or the reviewer needs independent passes. If it does, dispatch the 6 review-focused `team-*` workers per `## Fanout dispatch` below, then re-spawn `lead` with the workers' outputs and the `Dispatched-as:` map for synthesis. For small/low-risk diffs, `lead` writes `review.md` directly.
    - **Review artifact check.** Read the first line of `review.md` — if the file is missing or empty, re-spawn `lead` in review mode. Apply the verdict handling below only after this check passes.
    - Verdict `fix-required` and `cycles.review` ≤ 2 → back to `engineer` with findings; do not bump `cycles.test`.
    - `cycles.review` > 2 → escalate to user via `AskUserQuestion`. Print blocking findings + ask whether to continue, hand off, or abort.
12. **Security review (trigger-based).** Decide whether to fire:
    - Run `git -C <repo_root> diff --name-only` (if `repo_root` is set) or `git diff --name-only` (single-repo) to get changed paths. Fall back to the engineer's returned file list if git is unavailable.
    - If any path matches the sensitive-paths list (auth/session/token, password, crypto, SQL/query builder, raw HTML render, file/path handling, exec/shell, deserialise, env/secrets, new outbound network), fire it.
    - Also fire if the user requested it at the gate or via `revise` notes.
    - If firing: spawn `lead` in **security mode**, set `state.security_triggered=true`. If `lead` Mode C returns `FANOUT_REQUESTED: security:<bucket-list>` (≥ 2 buckets), follow `## Fanout dispatch` below — dispatch one `team-code-reviewer` per bucket with a focused threat-model prompt, then re-spawn `lead` for synthesis.
      - **Security artifact check.** Read the first line of `security.md` — if missing or empty, re-spawn `lead` in security mode before applying the verdict handling below.
      - Verdict `fix-required` with severity `high` → blocking. Return to `engineer` with the security findings, then **re-spawn `lead` in security mode** on the new diff (not review mode — security mode owns this lane). Each high-severity loop bumps `cycles.review` by 1; once `cycles.review > 2`, escalate the same way blocking review findings do.
      - Severity `medium` / `low` only → non-blocking; carry into `retro.md > Security findings (carry-over)` and proceed without a re-spawn.
    - If not firing: write a single line to `state.json` (`security_triggered=false`) and move on.
13. **Test.** Type-branch:
    - `feat` / `refactor` / `fix` → spawn `qa`. INDEX status → `testing`. State: `step=test, cycles.test++`. If `qa` returns `FANOUT_REQUESTED: test:<category-list>` (opt-in heuristic), follow `## Fanout dispatch` below — dispatch one `team-pr-test-analyzer` per category, then re-spawn `qa` for synthesis.
      - **Test artifact check.** Confirm `tests.md` exists (read first line). If absent, re-spawn `qa` before applying cycle logic.
      - `fix`: the prompt to qa must restate "verify the regression test fails on pre-fix code (use the test-commit vs fix-commit two-commit history; fall back to `git stash` or a scratch revert branch) and passes now."
      - Failing tests, `cycles.test` ≤ 3 → engineer fixes.
      - `cycles.test` > 3 → escalate via `AskUserQuestion`.
    - `chore` / `docs` → spawn `qa` with mode = `Skipped`; qa writes a one-line stub in `tests.md` explaining why and returns.
    - `spike` → skip entirely. Engineer's `recommendations.md` is the deliverable.
14. **Docs touch-up.** Spawn `engineer` in docs mode (skipped for `spike`; light for `fix`/`refactor`/`chore` — pass that hint).
15. **Ship.** Spawn `engineer` in ship mode. Pass `open_pr_on_ship` from state. Engineer stages, writes a commit message referencing the run ID + goal, and (if requested) opens a PR. Record commit hash + PR URL in `state.json` so `retro` can lift them.
    - **Ship check** (skip when `repo_root` is null or the engineer returned "no VCS — ship skipped"). Run `git -C <repo_root> log --oneline -1` and confirm the commit SHA the engineer reported is present. If the log shows no new commit, re-spawn `engineer` in ship mode.
    - Skipped for `spike` unless the user explicitly opted to commit at the gate.
16. **Retro.** Spawn `retro`. INDEX status → `done`, set finished date. State: `step=retro, next_step=skill-handoff`.
17. **Skill-candidate handoff.** Read `retro.md > Skill candidates`. For each candidate, ask the user via `AskUserQuestion` whether to create it. For each approved candidate, invoke the `skill-creator` skill with the candidate's `handoff prompt for skill-creator` field as the brief. Record outcomes in `retro.md > Skill candidates` (`status: created | skipped | deferred`).
18. **Done.** Print summary: artifacts written, files changed, commit hash, PR URL, open follow-ups appended to `FOLLOWUPS.md`, skills created. State: `phase=done, step=done`.

## Fanout dispatch

The `/dev` workflow's main agent and `pm`, `lead`, `qa`, and `engineer` sub-agents can request parallel team-agent fanout for the independent-sub-investigation case (spec research, plan integration points, review, security buckets, test categories, implement phases). The full pattern lives in `.claude/skills/fanout-team-agents/SKILL.md`; this section is the orchestrator's (main agent's) consumer-side contract.

### Recognising the signal

After every sub-agent return, scan the **first line** of the return: (a) case-insensitive `FANOUT_REQ` prefix → fanout signal (validate below); (b) `BLOCKER:` prefix → blocker (surface to user); (c) else → success. Before **advancing to the next step**, run the step's **Return check** if one is defined below. Return checks run **only** on the step's primary worker return (`pm` / `lead` / `engineer` / `qa` / `retro`) — **never** on intermediate `team-*` fanout workers, whose returns are collected for the synthesis re-spawn per `## Fanout dispatch` (the artifact doesn't exist until synthesis runs, so a check there would false-positive). A Return check is a presence/shape **tripwire**, not a quality review (that's the `lead` review and `qa` test gates) — it fires at most **one** corrective re-spawn; if the check still fails after that, escalate to the user via `AskUserQuestion` rather than looping. If a fanout signal is present, validate against the strict allowlist:

```text
^FANOUT_REQUESTED: (review|security:[a-z0-9,\-]+|plan:[a-z0-9,\-]+|test:[a-z0-9,\-]+|implement:[a-z0-9,\-]+|research:[a-z0-9,\-]+)$
```

If the first line matches the case-insensitive `FANOUT_REQ` prefix but does **not** match the strict regex (typos, casing, payload-shape errors), this is a **BLOCKER** — surface to the user via `AskUserQuestion` with the offending line and the 6 valid shapes. Do **not** silently fall through to non-fanout. The typo modes this catches: `FANOUTREQUESTED:` (missing underscore), `Fanout_Requested:` (case-mixed prefix), `FANOUT_REQUESTED:review` (no space after colon), `FANOUT_REQUESTED: REVIEW` (uppercase payload), `FANOUT_REQUESTED: review extra` (trailing junk), `FANOUT_REQUESTED: foo` (unknown payload).

### The 6 documented payload shapes

| Shape | Trigger phase / mode | Dispatch |
|-------|----------------------|----------|
| `FANOUT_REQUESTED: review` | Phase 2 step 11 — `lead` Mode B (condition-based: large, cross-module, critical, type/contract/test-sensitive, or uncertain review) | Spawn the 6 review-focused `team-*` workers against the diff |
| `FANOUT_REQUESTED: security:<bucket-list>` | Phase 2 step 12 — `lead` Mode C (opt-in, ≥ 2 buckets) | One `team-code-reviewer` per bucket with a focused threat-model prompt scoped to that bucket's paths |
| `FANOUT_REQUESTED: plan:<point-list>` | Phase 1 step 8 — `lead` Mode A (condition-based: unclear/high-risk S/M/L existing-code work; skip XS / pure-greenfield / straightforward changes) | For each point: one `team-codebase-explorer` pass for current state + one `team-best-practice-researcher` pass for current best practices |
| `FANOUT_REQUESTED: test:<category-list>` | Phase 2 step 13 — `qa` (opt-in, ≥ 2 of {unit, integration, e2e} AND any ≥ 3 tests) | One `team-pr-test-analyzer` per category |
| `FANOUT_REQUESTED: implement:<phase-list>` | Phase 2 step 10 — `engineer` Mode A (opt-in, L-tier with disjoint Files-touched) | One `engineer` per phase, then re-spawn the calling engineer for integration. **Caveat**: this shape races the Case 3 state.json discipline in `dev-agent-guard.sh`; treat as experimental until the guard is namespaced per-phase. |
| `FANOUT_REQUESTED: research:<question-list>` | Phase 1 step 7 — `pm` return-signal, plus step 6 spec-prep fanout from the main agent | One `team-codebase-explorer` per `codebase-*` question and one `team-best-practice-researcher` per `best-practice-*` question. pm cannot dispatch directly — pm returns `FANOUT_REQUESTED: research:<…>`, orchestrator dispatches, orchestrator re-spawns pm with the workers' findings appended to the interview Q&A. |

### The dispatch pattern (parallelism)

Once the signal validates, dispatch **all workers in the same orchestrator message** — Claude Code's `Agent` tool runs them concurrently when multiple invocations appear in one assistant turn. Sequential `Agent` calls across multiple turns are **not** parallel.

```
# In one orchestrator message:
Agent(subagent_type="team-codebase-explorer",        description="...", prompt=<focused-prompt-1>)
Agent(subagent_type="team-best-practice-researcher", description="...", prompt=<focused-prompt-2>)
Agent(subagent_type="team-code-reviewer",            description="...", prompt=<focused-prompt-3>)
Agent(subagent_type="team-code-simplifier",          description="...", prompt=<focused-prompt-4>)
Agent(subagent_type="team-comment-analyzer",         description="...", prompt=<focused-prompt-5>)
Agent(subagent_type="team-pr-test-analyzer",         description="...", prompt=<focused-prompt-6>)
Agent(subagent_type="team-silent-failure-hunter",    description="...", prompt=<focused-prompt-7>)
Agent(subagent_type="team-type-design-analyzer",     description="...", prompt=<focused-prompt-8>)
```

Each prompt is **self-contained** (the workers inherit none of the calling sub-agent's context). Include scope (paths / diff slice), goal (one sentence), constraints (what NOT to do), and output shape (the worker's documented section format from `.claude/agents/team-<role>.md`).

### Registry-not-refreshed fallback

If any `team-<role>` spawn fails with `Agent type 'team-<role>' not found`, the registry is session-scoped and the freshly-created `team-*.md` files are not yet discoverable. Two correct responses:

1. **Session restart** — close and re-open the Claude Code session so the registry picks up the new files; then retry `subagent_type="team-<role>"`.
2. **Inline fallback** — re-issue every worker spawn with `subagent_type="general-purpose"` and the worker's role contract read inline into the prompt: read `.claude/agents/team-<role>.md` end-to-end and prepend the body to the focused prompt. Parallelism is preserved (still one message, six `Agent(...)` calls). Record the actual `subagent_type` you used for each spawn — you'll need it for the synthesis step below. Full caveat in `.claude/skills/fanout-team-agents/SKILL.md > Operational caveats > Agent registry is session-scoped`.

### Re-spawn for synthesis

When every worker returns, re-spawn the calling sub-agent (`pm` / `lead` / `qa` / `engineer`) with:

- The workers' outputs concatenated into the prompt (one labelled block per worker).
- A `Dispatched-as:` map: one line per worker, `team-<role> → <actual subagent_type that ran>` (so the calling sub-agent can fill the mandatory `**Dispatched-as**:` provenance line on each `### team-<role>` subsection of the artifact — see `.workflow/_templates/review.md > Per-agent findings`).

The re-spawned sub-agent does the synthesis (spec discovery notes; plan current-state / research notes; per-agent sections + its own plan-adherence / AC / coverage / integration pass).

### Where fanout fires

The steps that can fire a fanout (signal originated by the main agent or spawned sub-agent, dispatched here):

- **Step 6 — Spec prep** — condition-based: main agent may dispatch `team-codebase-explorer` / `team-best-practice-researcher` before the interview when the intent names existing code, integration points, APIs, security-sensitive paths, unfamiliar domain terms, or 2+ independent research questions. Skip for XS pure-greenfield work with no unfamiliar domain/API choices.
- **Step 7 — Spec** — `pm` may return `FANOUT_REQUESTED: research:<question-list>` if it needs codebase or best-practice probes before writing `spec.md`.
- **Step 8 — Plan** — two paths. **(a) Push-based plan-prep (preferred for speed):** before the `lead` spawn, the orchestrator dispatches one `team-codebase-explorer` (haiku) per integration point in parallel for M/L existing-code work with ≥ 2 integration points, then spawns `lead` once with the findings — this keeps the slow LSP-walk off `lead`'s critical path. **(b) Pull-based fallback:** `lead` Mode A may still return `FANOUT_REQUESTED: plan:<point-list>` for anything prep did not cover (typically best-practice research); the orchestrator dispatches the residual workers and re-spawns `lead`.
- **Step 10 — Implement** — `engineer` may return `FANOUT_REQUESTED: implement:<phase-list>` (opt-in, L-tier only; see caveat above).
- **Step 11 — Review** — `lead` Mode B may return `FANOUT_REQUESTED: review` when the diff deserves independent specialist passes; otherwise it writes `review.md` directly.
- **Step 12 — Security review** — `lead` Mode C may return `FANOUT_REQUESTED: security:<bucket-list>` (opt-in when ≥ 2 buckets trip).
- **Step 13 — Test** — `qa` may return `FANOUT_REQUESTED: test:<category-list>` (opt-in heuristic).

## Cycle escalation

- Review cycles: max 2. Escalate clearly via `AskUserQuestion`: "Review still has blocking findings after cycle 2. Continue / hand off to me / abort?"
- Test cycles: max 3. Same pattern.
- Don't silently iterate beyond limits. Don't downgrade severity to fit.

## Rules

- Never invent requirements. Ambiguity → `AskUserQuestion` (≤4 questions per batch; one batch by default, a bounded dig loop of at most 3 narrowing batches when ambiguity is genuinely high — see step 6).
- **Never skip the interview.** A spec built from a one-line intent alone is a broken run. Step 6 is mandatory for every fresh run, even short intents.
- Never skip phases that the type matrix says should run. Skipping that's allowed by the matrix is recorded in `state.json > skipped_steps`.
- The gate is non-negotiable.
- **Never spawn an `orchestrator` sub-agent.** That sub-agent does not exist (and could not work — sub-agents can't spawn sub-agents). File-writing workflow `Agent` calls go to `pm`, `lead`, `engineer`, `qa`, or `retro`; fanout-only read/research calls may go to `team-*` workers.
- **Never fall back to `subagent_type: "general-purpose"` for /dev work.** Every file-writing step goes to one of the five named workers. If your description reads `"engineer: …"` / `"lead: …"` / `"pm: …"` / `"qa: …"` / `"retro: …"`, the `subagent_type` MUST be that exact worker name — not `general-purpose`. The `PreToolUse` hook at `.claude/hooks/dev-agent-guard.sh` enforces this and will block the call with a retry message. Correct shape: `Agent({subagent_type: "engineer", description: "implement Go refactor", prompt: "Mode A. Type=refactor. …"})`. The mode hint (`plan`/`review`/`security`, `implement`/`docs`/`ship`, etc.) goes in the *prompt*, not in the description.
- Keep your user-facing text to status updates: which phase, which agent, what's next. One sentence each. The exception is the gate summary, the interview batch, and the final summary, which are spec-shaped.
- End-of-turn: artifacts written + files changed + commit/PR + open follow-ups from `retro.md` + skills created. Nothing else.

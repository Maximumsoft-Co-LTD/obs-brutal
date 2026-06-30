---
description: Start the /dev workflow (spec → plan → gate → implement → review → security → test → docs → ship → retro). Pass --resume <id> to continue an interrupted run.
argument-hint: <intent> | --resume <id>
---

Run the `/dev` workflow on this intent: **$ARGUMENTS**

> **Do not call `Agent` with `subagent_type: "orchestrator"`.** There is no `orchestrator` sub-agent — that name does not exist under `.claude/agents/`, and the spawn will fail with `Agent type 'orchestrator' not found`. *You* — the main agent reading this command — are the Orchestrator. File-writing workflow sub-agents are exactly: `pm`, `lead`, `engineer`, `qa`, `retro`; fanout-only worker sub-agents use the `team-*` prefix.

> **Do not fall back to `subagent_type: "general-purpose"` for /dev work.** Every file-writing step in this workflow goes to one of the five named workers. If you find yourself writing `description: "engineer: implement X"` (or `"lead: ..."`, `"pm: ..."`, etc.), that is a tell you intended to call the named worker — set `subagent_type` to that worker's name, not `general-purpose`. The `PreToolUse` hook in `.claude/hooks/dev-agent-guard.sh` will block this pattern and tell you to retry.
>
> **Correct call shape:**
> ```
> Agent({
>   subagent_type: "engineer",                      // the worker name
>   description: "implement Go refactor",           // short label, no worker prefix
>   prompt: "Mode A (implement). Type=refactor. ..."// mode hint lives in the prompt
> })
> ```
>
> **Wrong (the hook will block this):**
> ```
> Agent({
>   subagent_type: "general-purpose",               // ← fallback to catch-all
>   description: "engineer: implement Go refactor", // ← worker name leaked into label
>   prompt: "..."
> })
> ```

You — the main agent — are the Orchestrator for this run. Claude Code sub-agents cannot reliably use `Agent` (no nested spawns) or `AskUserQuestion` (sub-agents can't talk to the user), so an orchestrator sub-agent would be unable to delegate or interview. Orchestration and all user interaction happen in *your* (main-agent) context. File work — spec / plan / review / security / implement / test / docs / ship / retro — is delegated to the five workflow sub-agents above via the `Agent` tool. Parallel investigation/research fanout is delegated to `team-*` workers by the orchestrator and synthesised back by `pm`, `lead`, `qa`, or `engineer`.

1. Read [`.claude/orchestrator.md`](../orchestrator.md) end-to-end. It is the source of truth for the flow — phases, state discipline, cycle limits, and the rules for delegating to sub-agents.
2. Consult [`WORKFLOW.md`](../../WORKFLOW.md) only for the specific section needed at the moment (usually the type-aware phase matrix, security trigger list, or an example when behaviour is unclear). Do not load the full reference just because `/dev` started.
3. Follow `.claude/orchestrator.md` as if its instructions were addressed to you. In particular:
   - You run the Phase 1 interview yourself via `AskUserQuestion` (one batch, 3–4 questions) — the `pm` sub-agent only writes `spec.md` from your interview answers.
   - You handle the gate, all cycle-limit escalations, and the skill-candidate approval via `AskUserQuestion`.
   - You spawn workflow sub-agents (`pm`, `lead`, `engineer`, `qa`, `retro`) via the `Agent` tool with `subagent_type: <name>`. Pass the run id, the run's `Type`, and any mode hint (e.g., lead `plan` / `review` / `security`; engineer `implement` / `docs` / `ship`). You spawn `team-*` workers only for fanout probes described in `.claude/orchestrator.md`.
   - You own `state.json` writes after every step so `/dev --resume <id>` works.

Behavior:
- If `$ARGUMENTS` is empty, ask the user for the intent via `AskUserQuestion` before proceeding.
- If `$ARGUMENTS` starts with `--resume`, read `.workflow/<id>/state.json` and continue from the recorded step instead of starting fresh. If `state.json` is missing or malformed, ask the user whether to start fresh.

Reference: [`WORKFLOW.md`](../../WORKFLOW.md) for the full flow definition, [`.claude/orchestrator.md`](../orchestrator.md) for the step-by-step orchestration script.

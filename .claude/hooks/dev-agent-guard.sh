#!/usr/bin/env bash
# PreToolUse guard for the /dev workflow.
#
# Catches three known failure modes when the orchestrator (main agent) delegates work:
#
#   1. subagent_type="orchestrator" — there is no orchestrator sub-agent;
#      the main agent IS the orchestrator. Spawn would fail with
#      "Agent type 'orchestrator' not found", but better to fail loud here
#      with the right pointer to the correct worker name.
#
#   2. subagent_type="general-purpose" while the description prefix targets
#      a /dev worker (pm, lead, engineer, qa, retro). This is the model
#      "knowing but not complying" — it labels the description with the
#      intended worker name but routes to the catch-all agent anyway.
#      Block and tell it to retry with the correct subagent_type.
#
#   3. state.json not updated between worker spawns. The companion
#      PostToolUse hook dev-state-mark.sh touches .last_worker_return
#      inside the active run dir when a worker returns. If the marker
#      is newer than state.json on the next worker spawn, the orchestrator
#      skipped the bookkeeping step prescribed by orchestrator.md > State
#      discipline. Block until state.json catches up — resume depends on it.
#
# This hook sits on the critical path of EVERY Agent spawn, so it does the least
# work possible: a single jq parse pulls tool_name + subagent_type (both
# newline-free tokens) in one process; the spawn-heavy `description` field is
# only parsed in Case 2, where it's actually needed. The Case 3 freshness check
# uses bash globbing + `-nt` (no subprocesses). Net: the common worker spawn
# pays for exactly one jq invocation, not three.

set -euo pipefail

input="$(cat)"

# One jq parse for the two fields every branch needs. `|| true` keeps the hook
# fail-open: a malformed payload yields empty fields and the guard waves it through.
IFS=$'\t' read -r tool_name subagent_type < <(
  printf '%s' "$input" | jq -r '[(.tool_name // ""), (.tool_input.subagent_type // "")] | @tsv'
) || true

[[ "$tool_name" != "Agent" ]] && exit 0

# Case 1: orchestrator is never a valid sub-agent
if [[ "$subagent_type" == "orchestrator" ]]; then
  jq -n '{
    decision: "block",
    reason: "BLOCKED by /dev guard: there is no `orchestrator` sub-agent. You — the main agent — ARE the orchestrator. Worker sub-agents you can spawn are: pm, lead, engineer, qa, retro (defined in .claude/agents/). Pick the right one and retry."
  }'
  exit 0
fi

# Case 2: general-purpose fallback with worker-name prefix in description.
# The description (which can contain newlines) is parsed only here, off the hot path.
if [[ "$subagent_type" == "general-purpose" ]]; then
  description="$(printf '%s' "$input" | jq -r '.tool_input.description // ""')"
  # case-insensitive match for the worker prefix
  desc_lc="$(printf '%s' "$description" | tr '[:upper:]' '[:lower:]')"
  for worker in pm lead engineer qa retro; do
    # match "<worker>:", "<worker> ", "<worker>(" at the start of the description
    if [[ "$desc_lc" =~ ^${worker}[[:space:]:\(] ]] || [[ "$desc_lc" == "$worker" ]]; then
      jq -n --arg w "$worker" '{
        decision: "block",
        reason: ("BLOCKED by /dev guard: you set subagent_type=\"general-purpose\" but the description starts with \"\($w)\" — that signals you intended to spawn the `\($w)` worker agent. The /dev workflow worker agents live in .claude/agents/ and must be called by name. Retry with subagent_type=\"\($w)\" instead. (Worker agents: pm, lead, engineer, qa, retro.)")
      }'
      exit 0
    fi
  done
fi

# Case 3: state.json must be updated between /dev worker spawns
case "$subagent_type" in
  pm|lead|engineer|qa|retro)
    PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"
    WF_DIR="$PROJECT_DIR/.workflow"
    if [[ -d "$WF_DIR" ]]; then
      latest_state=""
      for f in "$WF_DIR"/*/state.json; do
        [[ -f "$f" ]] || continue
        if [[ -z "$latest_state" ]] || [[ "$f" -nt "$latest_state" ]]; then
          latest_state="$f"
        fi
      done
      if [[ -n "$latest_state" ]]; then
        run_dir="$(dirname "$latest_state")"
        run_id="$(basename "$run_dir")"
        marker="$run_dir/.last_worker_return"
        # Block only if both files exist AND marker is newer. If marker is
        # missing, this is the first worker spawn of the run — let it through.
        if [[ -f "$marker" ]] && [[ "$marker" -nt "$latest_state" ]]; then
          state_rel=".workflow/$run_id/state.json"
          reason="BLOCKED by /dev guard: $state_rel was not updated after the last worker returned. orchestrator.md > State discipline requires writing state.json (phase, step, next_step, cycles, last_updated, last_agent) after EVERY step, before spawning the next worker — otherwise /dev --resume $run_id is broken. Update $state_rel via Write/Edit, then retry the spawn."
          jq -n --arg reason "$reason" '{decision: "block", reason: $reason}'
          exit 0
        fi
      fi
    fi
    ;;
esac

exit 0

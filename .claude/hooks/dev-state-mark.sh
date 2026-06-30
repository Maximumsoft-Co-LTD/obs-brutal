#!/usr/bin/env bash
# PostToolUse marker for the /dev workflow.
#
# When a /dev worker sub-agent (pm | lead | engineer | qa | retro) returns:
#   1. Touch a marker file in the most-recently-modified .workflow/<id>/ dir.
#      The companion PreToolUse check in dev-agent-guard.sh refuses to spawn
#      the next worker until state.json has been updated past that marker.
#   2. Emit an additionalContext reminder back to the orchestrator (main
#      agent) so it doesn't forget the state.json write between worker
#      returns. The reminder is what closes the loop on
#      orchestrator.md > State discipline — without it the rule is prose
#      only and gets forgotten in the moment.

set -euo pipefail

# jq is the only hard dependency. Fail open if it's missing — better to lose
# the marker than to break the user's Agent call with a non-/dev failure.
command -v jq >/dev/null 2>&1 || exit 0

input="$(cat)"

tool_name="$(printf '%s' "$input" | jq -r '.tool_name // ""')"
[[ "$tool_name" == "Agent" ]] || exit 0

subagent_type="$(printf '%s' "$input" | jq -r '.tool_input.subagent_type // ""')"

# Only mark for /dev worker returns. Other Agent spawns are unrelated and
# their post-hooks shouldn't poison the next /dev worker's pre-check.
case "$subagent_type" in
  pm|lead|engineer|qa|retro) ;;
  *) exit 0 ;;
esac

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"
WF_DIR="$PROJECT_DIR/.workflow"
[[ -d "$WF_DIR" ]] || exit 0

# Find the most recently modified .workflow/<id>/state.json — that's the
# active /dev run. If there isn't one, this Agent call isn't part of /dev.
latest_state=""
for f in "$WF_DIR"/*/state.json; do
  [[ -f "$f" ]] || continue
  if [[ -z "$latest_state" ]] || [[ "$f" -nt "$latest_state" ]]; then
    latest_state="$f"
  fi
done

[[ -n "$latest_state" ]] || exit 0

run_dir="$(dirname "$latest_state")"
run_id="$(basename "$run_dir")"
touch "$run_dir/.last_worker_return"

# Read the current step from state.json so the reminder can name what's
# expected next. Fail-soft: if the file is malformed, omit the step hint.
cur_phase="$(jq -r '.phase // ""' "$latest_state" 2>/dev/null || printf '')"
cur_step="$(jq -r '.step // ""' "$latest_state" 2>/dev/null || printf '')"

reminder=$(cat <<EOF
[/dev state discipline] Worker \`${subagent_type}\` just returned for run \`${run_id}\` (phase=${cur_phase:-?}, step=${cur_step:-?}).

BEFORE your next \`Agent\` spawn, update \`.workflow/${run_id}/state.json\` via Write/Edit with:
  - \`phase\`: current phase
  - \`step\`: the step you just completed
  - \`next_step\`: per the type matrix in WORKFLOW.md (use \`"skipped:<reason>"\` for matrix-skipped steps)
  - \`cycles.review\` / \`cycles.test\`: bump only on actual cycle increments
  - \`last_updated\`: fresh ISO timestamp
  - \`last_agent\`: \`${subagent_type}\`

The PreToolUse guard (\`.claude/hooks/dev-agent-guard.sh\` Case 3) blocks the next worker spawn until the mtime of \`state.json\` is newer than \`.last_worker_return\`. Skipping this is the single most common reason a /dev run gets BLOCKED and needs \`/dev --resume\`.
EOF
)

jq -n --arg ctx "$reminder" '{
  hookSpecificOutput: {
    hookEventName: "PostToolUse",
    additionalContext: $ctx
  }
}'

exit 0

#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)

run_bg() {
  name=$1; shift
  cmd=$*
  echo "[RUN] $name: $cmd" >&2
  bash -lc "$cmd" >"$ROOT_DIR/logs/$name.out" 2>"$ROOT_DIR/logs/$name.err" &
  echo $!
}

mkdir -p "$ROOT_DIR/logs"

declare -A pids

# Start examples (non-blocking)
pids[otel_loki]=$(run_bg otel_loki "go run ./examples/otel_loki")
pids[http]=$(run_bg http "go run ./examples/http")
pids[gin]=$(run_bg gin "go run ./examples/gin")
pids[mongo]=$(run_bg mongo "go run ./examples/mongo")
pids[redis]=$(run_bg redis "go run ./examples/redis")
pids[amqp]=$(run_bg amqp "go run ./examples/amqp")
pids[basic]=$(run_bg basic "go run ./examples/basic")
pids[cron]=$(run_bg cron "go run ./examples/cron")

sleep 5

echo "\nSUMMARY:" >&2
for name in "${!pids[@]}"; do
  pid=${pids[$name]}
  if ps -p "$pid" > /dev/null 2>&1; then
    echo "- $name: RUNNING (pid=$pid)" >&2
  else
    echo "- $name: EXITED (see logs/$name.*)" >&2
  fi
done

echo "Done (logs under logs/)." >&2


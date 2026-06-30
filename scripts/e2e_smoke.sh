#!/usr/bin/env bash
# Run the boeng demo examples in the background and dump their stdout/stderr
# under logs/<name>.{out,err}. Useful for eyeballing JSON output shape and
# verifying that the binary actually flushes async sinks on Close.
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

# Start the three boeng usage modes (non-blocking)
pids[boeng]=$(run_bg boeng "go run ./examples/boeng")
pids[boeng_ctx]=$(run_bg boeng_ctx "go run ./examples/boeng_ctx")
pids[boeng_legacy]=$(run_bg boeng_legacy "go run ./examples/boeng_legacy")

sleep 5

echo "" >&2
echo "SUMMARY:" >&2
for name in "${!pids[@]}"; do
  pid=${pids[$name]}
  if ps -p "$pid" > /dev/null 2>&1; then
    echo "- $name: RUNNING (pid=$pid)" >&2
  else
    echo "- $name: EXITED (see logs/$name.*)" >&2
  fi
done

echo "Done (logs under logs/)." >&2

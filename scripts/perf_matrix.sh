#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

TOTAL=${TOTAL:-300000}
WORKERS=${WORKERS:-cpu}
MODES=${MODES:-unified,async,strategy}
STRUCTURED=${STRUCTURED:-false}

SINKS=${SINKS:-devnull,buffer,file}

echo "Running perf matrix: total=$TOTAL workers=$WORKERS modes=$MODES structured=$STRUCTURED sinks=$SINKS" 1>&2

IFS=',' read -r -a SINK_ARR <<< "$SINKS"

for SINK in "${SINK_ARR[@]}"; do
  case "$SINK" in
    devnull)
      CMD=(go run ./cmd/perf_runner -summary -total="$TOTAL" -workers="$WORKERS" -modes="$MODES" -structured="$STRUCTURED" -sink=devnull)
      ;;
    buffer)
      CMD=(go run ./cmd/perf_runner -summary -total="$TOTAL" -workers="$WORKERS" -modes="$MODES" -structured="$STRUCTURED" -sink=buffer -buffer_size=50000 -buffer_timeout=50ms)
      ;;
    file)
      mkdir -p logs >/dev/null 2>&1 || true
      CMD=(go run ./cmd/perf_runner -summary -total="$TOTAL" -workers="$WORKERS" -modes="$MODES" -structured="$STRUCTURED" -sink=file -file=logs/perf.log)
      ;;
    *)
      echo "Unknown sink: $SINK" 1>&2; exit 1;
      ;;
  esac

  echo "\n=== Sink: $SINK ===" 1>&2
  # Drop stdout to avoid flooding output; only stderr summary is kept
  "${CMD[@]}" >/dev/null
done



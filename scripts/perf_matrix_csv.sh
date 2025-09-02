#!/usr/bin/env bash
set -euo pipefail

OUT_CSV=${1:-perf_results.csv}
OUT_MD=${2:-perf_results.md}

# Progress helpers (print to stderr so CSV/MD stay clean)
current_step=0
baseline_total=$((2*3))   # structured: false/true (2) × workers: 1,cpu,2cpu (3)
buffer_total=3            # bs: 500,1000,2000
file_total=1
net_total=2               # netmock + netmock_batch
total_steps=$((baseline_total + buffer_total + file_total + net_total))

progress() {
  current_step=$((current_step+1))
  ts=$(date +%H:%M:%S)
  pct=$(( 100 * current_step / total_steps ))
  echo "[$ts] ($current_step/$total_steps, ${pct}%) $*" >&2
}

header="scenario,workers,structured,sink,logs_per_sec,per_log_us,notes"
echo "$header" > "$OUT_CSV"
echo "| scenario | workers | structured | sink | logs/sec | per_log_us | notes |" > "$OUT_MD"
echo "|---|---:|:---:|---|---:|---:|---|" >> "$OUT_MD"

append() {
  local scenario=$1; shift
  local workers=$1; shift
  local structured=$1; shift
  local sink=$1; shift
  local lps=$1; shift
  local us=$1; shift
  local notes=${1:-}
  echo "$scenario,$workers,$structured,$sink,$lps,$us,$notes" >> "$OUT_CSV"
  echo "| $scenario | $workers | $structured | $sink | $lps | $us | $notes |" >> "$OUT_MD"
}

run_case() {
  local total=$1 workers=$2 structured=$3 sink=$4 args=$5 notes=$6
  progress "running: total=$total workers=$workers structured=$structured sink=$sink ${args:+args=$args}"
  out=$(go run ./cmd/perf_runner -summary -total=$total -workers=$workers -modes=unified,async -structured=$structured -sink=$sink $args 2>&1 | tail -n +1)
  # lines look like: "unified    | 14.5ms | logs/sec=... | per_log_us=..."
  u_lps=$(echo "$out" | awk '/^unified[[:space:]]/{print $5}' | sed 's/logs\/sec=//')
  u_us=$(echo "$out" | awk '/^unified[[:space:]]/{print $7}' | sed 's/per_log_us=//')
  a_lps=$(echo "$out" | awk '/^async[[:space:]]/{print $5}'   | sed 's/logs\/sec=//')
  a_us=$(echo "$out" | awk '/^async[[:space:]]/{print $7}'    | sed 's/per_log_us=//')
  append unified $workers $structured $sink $u_lps $u_us "$notes"
  append async   $workers $structured $sink $a_lps $a_us "$notes"
}

# Baseline devnull for structured=false/true
for s in false true; do
  for w in 1 cpu 2cpu; do
    run_case 200000 $w $s devnull "" baseline
  done
done

# Buffered async (false)
for bs in 500 1000 2000; do
  progress "running: buffer sink (bs=$bs)"
  out=$(go run ./cmd/perf_runner -summary -total=200000 -workers=cpu -modes=async -structured=false -sink=buffer -buffer_size=$bs -buffer_timeout=100ms 2>&1 | tail -n +1)
  a_lps=$(echo "$out" | awk '/^async[[:space:]]/{print $5}' | sed 's/logs\/sec=//')
  a_us=$(echo "$out" | awk '/^async[[:space:]]/{print $7}' | sed 's/per_log_us=//')
  append async cpu false buffer\(bs=$bs\) $a_lps $a_us "tune buffer"
done

# File sink
progress "running: file sink (lumberjack optimal)"
out=$(go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=async -structured=false -sink=file -file=logs/perf.log 2>&1 | tail -n +1)
a_lps=$(echo "$out" | awk '/^async[[:space:]]/{print $5}' | sed 's/logs\/sec=//')
a_us=$(echo "$out" | awk '/^async[[:space:]]/{print $7}' | sed 's/per_log_us=//')
append async cpu false file $a_lps $a_us "file IO"

# Network mocks
progress "running: netmock (1ms per write)"
out=$(go run ./cmd/perf_runner -summary -total=50000 -workers=cpu -modes=async -structured=false -sink=netmock -net_latency=1ms 2>&1 | tail -n +1)
a_lps=$(echo "$out" | awk '/^async[[:space:]]/{print $5}' | sed 's/logs\/sec=//')
a_us=$(echo "$out" | awk '/^async[[:space:]]/{print $7}' | sed 's/per_log_us=//')
append async cpu false netmock\(1ms\) $a_lps $a_us "network latency"

progress "running: netmock_batch (1ms per batch, bs=1000)"
out=$(go run ./cmd/perf_runner -summary -total=100000 -workers=cpu -modes=async -structured=false -sink=netmock_batch -net_latency=1ms -net_batch_size=1000 -net_batch_timeout=100ms 2>&1 | tail -n +1)
a_lps=$(echo "$out" | awk '/^async[[:space:]]/{print $5}' | sed 's/logs\/sec=//')
a_us=$(echo "$out" | awk '/^async[[:space:]]/{print $7}' | sed 's/per_log_us=//')
append async cpu false netmock_batch\(1ms,bs=1000\) $a_lps $a_us "network batch"

echo "[DONE] Wrote $OUT_CSV and $OUT_MD" >&2

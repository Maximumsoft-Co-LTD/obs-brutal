#!/usr/bin/env bash
set -euo pipefail

run() {
  echo "$@" >&2
  eval "$@"
}

echo "scenario | workers | structured | sink | logs/sec | per_log_us | notes"

# Baseline devnull
for s in false true; do
  for w in 1 cpu 2cpu; do
    out=$(go run ./cmd/perf_runner -summary -total=300000 -workers=$w -modes=unified,async -structured=$s -sink=devnull 2>&1 | tail -n +1)
    unified=$(echo "$out" | grep "^unified" | awk '{print $1,$3,$5,$7}' | sed 's/|//g')
    async=$(echo "$out" | grep "^async" | awk '{print $1,$3,$5,$7}' | sed 's/|//g')
    u_lps=$(echo $unified | awk '{print $2}')
    u_us=$(echo $unified | awk '{print $3}')
    a_lps=$(echo $async | awk '{print $2}')
    a_us=$(echo $async | awk '{print $3}')
    echo "unified | $w | $s | devnull | $u_lps | $u_us | baseline"
    echo "async   | $w | $s | devnull | $a_lps | $a_us | baseline"
  done
done

# Buffered async (vary buffer)
for bs in 500 1000 2000; do
  out=$(go run ./cmd/perf_runner -summary -total=300000 -workers=cpu -modes=async -structured=false -sink=buffer -buffer_size=$bs -buffer_timeout=100ms 2>&1 | tail -n +1)
  async=$(echo "$out" | grep "^async" | awk '{print $1,$3,$5,$7}' | sed 's/|//g')
  a_lps=$(echo $async | awk '{print $2}')
  a_us=$(echo $async | awk '{print $3}')
  echo "async   | cpu | false | buffer(bs=$bs) | $a_lps | $a_us | tune buffer"
done

# File sink (lumberjack optimal file sink)
out=$(go run ./cmd/perf_runner -summary -total=200000 -workers=cpu -modes=async -structured=false -sink=file -file=logs/perf.log 2>&1 | tail -n +1)
async=$(echo "$out" | grep "^async" | awk '{print $1,$3,$5,$7}' | sed 's/|//g')
a_lps=$(echo $async | awk '{print $2}')
a_us=$(echo $async | awk '{print $3}')
echo "async   | cpu | false | file | $a_lps | $a_us | file IO"

# Network mock (simulate 1ms latency)
out=$(go run ./cmd/perf_runner -summary -total=50000 -workers=cpu -modes=async -structured=false -sink=netmock -net_latency=1ms 2>&1 | tail -n +1)
async=$(echo "$out" | grep "^async" | awk '{print $1,$3,$5,$7}' | sed 's/|//g')
a_lps=$(echo $async | awk '{print $2}')
a_us=$(echo $async | awk '{print $3}')
echo "async   | cpu | false | netmock(1ms) | $a_lps | $a_us | network latency"


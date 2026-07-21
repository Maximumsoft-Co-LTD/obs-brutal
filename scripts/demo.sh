#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)

usage() {
  cat <<'USAGE'
Demo runner for obs-brutal

Usage:
  scripts/demo.sh up   -p [mini|file|full] [-e all|otel_loki|promtail_file|none]
  scripts/demo.sh down -p [mini|file|full]

Profiles:
  mini  - Loki + Grafana (lightweight). Example pushes logs directly to Loki.
  file  - Promtail + Loki + Grafana. Example writes to ./logs/ for Promtail to ship.
  full  - Full stack (Prometheus, Tempo, Jaeger, OTEL Collector, Loki, Grafana).

Examples run (default mapping):
  mini -> go run ./examples/demo_all -svc mini-demo -env dev -http=false -loki=true
  file -> go run ./examples/promtail_file
  full -> go run ./examples/otel_loki
Override example with -e/--example:
  all           -> go run ./examples/demo_all (flags auto-adjusted per profile)
  otel_loki     -> go run ./examples/otel_loki
  promtail_file -> go run ./examples/promtail_file
  none          -> do not run any example (stack only)

USAGE
}

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "missing command: $1" >&2; exit 1; }; }

compose() { ( cd "$ROOT_DIR/compose" && docker compose "$@" ); }

wait_http() {
  local url="$1"; local tries=${2:-30}; local sleep_s=${3:-1}
  while (( tries > 0 )); do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    tries=$((tries-1)); sleep "$sleep_s"
  done
  echo "Timeout waiting for $url" >&2; return 1
}

up() {
  local profile="$1"; local example_override="${2:-}"
  echo "[demo] starting profile=$profile" >&2
  need_cmd docker; need_cmd curl; need_cmd go
  compose --profile "$profile" up -d

  # Basic readiness checks per profile
  case "$profile" in
    mini|file)
      wait_http "http://localhost:3100/ready" 60 1 || true
      wait_http "http://localhost:3000/login" 60 1 || true
      ;;
    full)
      wait_http "http://localhost:3100/ready" 60 1 || true
      wait_http "http://localhost:3000/login" 60 1 || true
      wait_http "http://localhost:16686" 60 1 || true
      wait_http "http://localhost:8889/metrics" 60 1 || true
      ;;
  esac

  # Run example matching the profile
  echo "[demo] running example for profile=$profile${example_override:+ (override=$example_override)}" >&2
  if [[ -n "$example_override" ]]; then
    case "$example_override" in
      none)
        echo "[demo] skip running example (stack only)" >&2
        ;;
      all)
        case "$profile" in
          mini) ( cd "$ROOT_DIR" && go run ./examples/demo_all -svc mini-demo -env dev -http=false -loki=true -ch=false -file=false ) ;;
          file) ( cd "$ROOT_DIR" && go run ./examples/demo_all -svc file-demo -env dev -http=true -file=true -loki=false -ch=false ) ;;
          full) ( cd "$ROOT_DIR" && go run ./examples/demo_all -svc full-demo -env dev -http=true -loki=true -ch=true ) ;;
          *) echo "unknown profile: $profile" >&2; exit 1;;
        esac
        ;;
      otel_loki)
        ( cd "$ROOT_DIR" && go run ./examples/otel_loki )
        ;;
      promtail_file)
        ( cd "$ROOT_DIR" && go run ./examples/promtail_file )
        ;;
      *) echo "unknown example: $example_override" >&2; exit 1;;
    esac
  else
    case "$profile" in
      mini)
        ( cd "$ROOT_DIR" && go run ./examples/demo_all -svc mini-demo -env dev -http=false -loki=true )
        ;;
      file)
        ( cd "$ROOT_DIR" && go run ./examples/promtail_file )
        ;;
      full)
        ( cd "$ROOT_DIR" && go run ./examples/otel_loki )
        ;;
      *) echo "unknown profile: $profile" >&2; exit 1;;
    esac
  fi

  echo "\n[demo] Open Grafana:   http://localhost:3000 (admin/admin)" >&2
  if [[ "$profile" == "file" ]]; then
    echo "[demo] Loki query (Explore): {job=\"obs-brutal\"}" >&2
  else
    echo "[demo] Loki query (Explore): {app=\"obs-brutal\"}" >&2
  fi
  if [[ "$profile" == "full" ]]; then
    echo "[demo] Jaeger UI:      http://localhost:16686" >&2
    echo "[demo] Metrics (OTEL): curl http://localhost:8889/metrics | rg obs_async_" >&2
  fi
}

down() {
  local profile="$1"
  echo "[demo] stopping profile=$profile" >&2
  need_cmd docker
  compose --profile "$profile" down -v || true
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    up|down) ;;
    *) usage; exit 1;;
  esac
  shift || true

  local profile="mini"
  local example=""
  while (( "$#" )); do
    case "$1" in
      -p|--profile) profile="${2:-mini}"; shift 2;;
      -e|--example) example="${2:-}"; shift 2;;
      *) usage; exit 1;;
    esac
  done

  case "$cmd" in
    up) up "$profile" "$example";;
    down) down "$profile";;
  esac
}

main "$@"

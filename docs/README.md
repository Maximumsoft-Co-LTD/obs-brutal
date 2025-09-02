# Architecture (Hexagonal)

## Layout

```
internal/
  core/
    domain/          # Domain models (LogEntry, Level)
    port/            # Ports (interfaces) – Sink, TelemetryProvider, strategies
    service/         # Application services (Unified/Async/Strategy/Security + OTEL logger orchestrator)
      base/          # Unified/Async loggers (pure core)
      strategy/      # Strategy manager + built-in strategies
      security/      # PII masking, audit trail, access control
      otel_logger.go # OTEL-enabled logger (uses port.TelemetryProvider)
  adapter/
    inbound/         # HTTP (Gin) response helpers
    outbound/
      otel/          # OTEL provider (Tracer/Meter) + wiring helpers
      sink/*         # stdout/json/file/lumberjack/buffered/loki/alerts/...

logtrc/              # Public facade API (constructors + helpers)
examples/            # runnable examples
compose/             # observability stack
```

## Dependency Direction

```
   Ports ← Service → Adapters
     ^        ^          ^
     |        |          |
   Domain ────┴──────────┘
```

- Domain: pure types (`LogEntry`, `Level`)
- Port: Sink/TelemetryProvider/Strategy interfaces
- Service: logbrut core (Unified/Async), strategy manager + built-ins, OTEL logger orchestrator; depends only on Port/Domain
- Adapters: concrete providers/sinks (OTEL, stdout/file/loki/alerts, etc.) + inbound HTTP helpers
- Facade (`logtrc`): routes constructors to core/adapters; exposes helpers like NewOTelWithService/NewAsyncWithOTel

## Notes
- No import cycles: Service depends only on Port/Domain; Adapters depend on Port/Service
- OTEL provider lives in adapter; core takes a TelemetryProvider
- Examples use util.WithTraceID/WithUserID/WithRequestID to propagate IDs safely
- Metrics: Async pipelines report via delta counters + queue-size histogram when TelemetryProvider is available

## Strategy Toggle Examples

Toggle strategies via CLI (perf_runner):

```bash
# Baseline strategy (all enabled by default)
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull

# Disable masking
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull -strategy_no_mask=true

# Disable sampling
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull -strategy_no_sample=true

# Disable filtering (default true)
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull -strategy_no_filter=true
```

Toggle in code (runtime):

```go
import (
    service "obs-brutal/internal/core/service"
    "obs-brutal/internal/core/domain"
)

s := service.NewStrategyLogBrt(domain.InfoLevel)
// Remove specific strategies by kind/name (see strategies.go names)
s.RemoveStrategy("sampler", "rate_sampler")
s.RemoveStrategy("masker",  "regex_masker")
s.RemoveStrategy("filter",  "level_filter")
// Re-add if needed
s.AddSampler(service.NewRateSampler(0.5))
```

## Performance Tuning (Guidelines)

Defaults to start with:

- Async + Buffered: `buffer_size=1000`, `buffer_timeout=50–100ms`
- File (Lumberjack) + Buffered: `rotate_size_bytes=10–50MB`, `max_backups=7–14`, `compress=true`
- Network (Loki/OTLP): Async + batching/backoff; queue sized for peak; explicit drop policy
- Hygiene: avoid `fmt.Sprintf` in hot path; limit fields; filter before sampler

Observability metrics (OTEL hooks are built in):

- Counters (delta): `obs_async_processed_total`, `obs_async_dropped_total`, `obs_async_batches_total`
- Histogram: `obs_async_queue_size`

Alerting suggestions:

- Dropped logs sustained (≥ 1m): `increase(obs_async_dropped_total[1m]) > 0`
- Backlog sustained (≥ 1m): `avg_over_time(obs_async_queue_size[1m]) / queue_capacity >= 0.8`
- Throughput anomaly: processed delta drops under SLO baseline for ≥ 5m

## Monitoring & Alerting (Prometheus + Alertmanager)

PromQL alerts (examples):

```promql
# Dropped logs sustained (>0 deltas over 1m)
increase(obs_async_dropped_total[1m]) > 0

# Queue backlog sustained (>= 80% capacity)
avg_over_time(obs_async_queue_size[1m]) / queue_capacity >= 0.8

# Throughput anomaly: 5m rate below SLO baseline
(
  rate(obs_async_processed_total[5m])
) < slo_processed_rate
```

Alertmanager (example routes/receivers):

```yaml
route:
  group_by: ['alertname','service']
  group_wait: 10s
  group_interval: 1m
  repeat_interval: 30m
  receiver: 'ops'
  routes:
  - match:
      severity: critical
    receiver: 'paging'

receivers:
  - name: 'ops'
    slack_configs:
      - channel: '#ops'
        send_resolved: true
  - name: 'paging'
    pagerduty_configs:
      - routing_key: ${PAGERDUTY_KEY}
```

Notes:
- Add `service` label to metric labels to route per service (NewOTelWithService sets it)
- Define `queue_capacity` and `slo_processed_rate` as recording rules per service

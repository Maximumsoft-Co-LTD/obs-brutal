## obs-brutal

High-performance logging and tracing toolkit for Go with clean APIs, optional OpenTelemetry, strategy-based filtering/sampling/masking, async pipeline, and web-friendly helpers.

### Highlights
- **Unified logbrut core**: fast structured logging with fluent chaining
- **Async pipeline**: high-throughput non-blocking logging
- **Strategy engine**: filter by level/module, sample by rate/adaptive, PII masking
- **OTEL integration**: tracing + metrics + Gin middleware (optional)
- **Security mode**: field-level masking and audit trail (optional)
- **Web facade (`logtrc`)**: small public surface, sensible defaults

## Install
```bash
go get obs-brutal
```

## Quick Start (basic)
```go
package main

import "obs-brutal/logtrc"

func main() {
    log := logtrc.NewDefault()
    log.Info("hello world")
    log.F("user_id", 123).Info("structured")
}
```

## Quick Setup (OTEL + Loki + Zerolog)
```go
package main

import (
    "obs-brutal/logtrc"
)

func main() {
    sinks := []logtrc.Sink{
        logtrc.NewConsoleSink(true),
        logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{"app": "obs-brutal", "env": "dev"}),
        logtrc.NewZerologSink(),
    }
    var logger logtrc.LogBrt
    if ot, _, err := logtrc.NewOTelWithService("quicksetup", "1.0.0", "dev", "localhost:4317", logtrc.INFO, sinks...); err == nil && ot != nil {
        logger = ot
    } else {
        logger = logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)
    }
    logger.F("module", "quicksetup").F("environment", "dev").Info("obs-brutal ready")
}
```
Run the combined example:
```bash
go run ./examples/otel_loki
```

## Console Toggle
เปิด/ปิดการพิมพ์ลงเทอร์มินัลแบบ runtime-safe โดยไม่กระทบ sink อื่น แม้ปลายทางภายนอกล่ม
```go
console := logtrc.NewConsoleSink(true) // true=on, false=off
_ = console.Configure(map[string]interface{}{"enabled": false}) // ปิดภายหลัง
```

## Full Demo (all methods)
ครอบคลุม Fluent API ทั้งหมด + Strategy + Security + OTLP + Loki + Zerolog + AMQP propagation
```bash
go run ./examples/full_demo
```

## Method Reference (fluent API)
```go
// Fields
log.F("key", "val").Info("msg")
log.Fs(map[string]interface{}{"a":1,"b":true}).Info("msg")

// Context & IDs
log.Ctx(ctx).Info("with context")
log.TraceID("trace-hex").SpanID("span-hex") // if available
log.UserID("u-1").RequestID("req-1").Info("ids")

// Errors
log.WithError(err).Error("failed")

// Levels
log.Debug("msg"); log.Info("msg"); log.Warn("msg"); log.Error("msg")

// Formatted
log.Debugf("v=%d", 1); log.Infof("%s", "ok")
```

## Strategy (filters/samplers/maskers)
```go
s := core.NewStrategyLogBrt(core.INFO, logtrc.NewFastStdoutSink())
s.AddFilter(core.NewLevelFilter(core.INFO, core.ERROR))
s.AddSampler(core.NewRateSampler(0.25))
s.AddMasker(core.NewPIIMasker())
s.F("email","john@doe.com").Info("masked")
```

## Security (PII masking + access control + audit)
```go
sec, _ := logtrc.NewSecurityLogBrt("svc","1.0.0","prod","jaeger:4317", logtrc.INFO)
sec.F("email","john@doe.com").Info("masked output")
```

## Compose Stack (Observability)
อยู่ที่ `compose/`:
- Grafana: 3000, Prometheus: 9090, Loki: 3100, Tempo: 3200, Jaeger: 16686, OTLP: 4317, ClickHouse: 8123
```bash
cd compose && docker compose up -d
```

## Sinks (outputs)
```go
stdout := logtrc.NewFastStdoutSink()
json   := logtrc.NewJSONSink()
file   := logtrc.NewOptimalFileSink()
// configure rotation (optional)
_ = file.Configure(map[string]interface{}{
    "filename":           "logs/app.log",
    "rotate_size_bytes":  10 << 20, // 10MB
    "max_backups":        5,
})

// Production-grade rotation (lumberjack)
lj := logtrc.NewLumberjackSink()
_ = lj.Configure(map[string]interface{}{
    "filename":"logs/app.log", "max_size_mb":100, "max_backups":14, "max_age_days":14, "compress":true,
})

// ClickHouse (HTTP JSONEachRow)
ch := logtrc.NewClickHouseSink()
_ = ch.Configure(map[string]interface{}{
    "endpoint":"http://localhost:8123", "database":"obs", "table":"logs",
    "username":"default", "password":"", "auto_create":true,
})

log := logtrc.New(logtrc.LogLevel(logtrc.INFO))
log.With("sink", stdout.Name()).Info("ok")

// Wrap any sink with a buffer (batching I/O)
buf := logtrc.NewBufferedWrap(file, 1000, 100*time.Millisecond)
_ = buf // use in sinks list
```

### Multi-sinks (tee หลายปลายทางพร้อมกัน)
ส่งออกหลายช่องทางพร้อมกันได้ โดยใส่หลาย `Sink` ตอนสร้าง logbrut
```go
sinks := []logtrc.Sink{
  logtrc.NewConsoleSink(false),
  logtrc.NewLumberjackSink(),
  // Wrap file with buffer
  logtrc.NewBufferedWrap(logtrc.NewLumberjackSink(), 2000, 100*time.Millisecond),
  logtrc.NewLokiPushSink("http://loki:3100/loki/api/v1/push", map[string]string{"app":"svc","env":"prod"}),
  logtrc.NewClickHouseSink(),
}
log := logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)
log.F("module","demo").Info("multi-sinks tee")
```

Loki advanced config via Configure:
```go
lk := logtrc.NewLokiPushSink("http://loki:3100/loki/api/v1/push", nil)
_ = lk.Configure(map[string]interface{}{
  "batch_size":   200,
  "timeout_ms":   100,
  "max_retries":  3,
  "retry_base_ms": 100,
})
```

## ClickHouse (SQL examples)

## Project Structure (Hexagonal)

- `internal/core/domain`: Domain models and pure types
- `internal/core/port`: Ports (interfaces) for services/adapters
- `internal/core/service`: Business logic (unified/async/strategy/otel loggers)
  - `log/`: Smart LogTrc + ResponseBuilder for HTTP (Gin)
  - `json/`: JSON writer helpers for `LogEntry`
  - `strategy/`: Strategy interfaces + manager
  - `otel/`: OTEL Provider (Tracer/Meter/metrics helper)
  - `security/`: PII masking, audit trail, access control, `SecurityLogBrt`
- `internal/adapter/...`: Inbound/Outbound adapters (HTTP middlewares, sinks, OTEL adapter facade)

Principles:
- Adapters never touch service internals; they depend on `port` + public `service` API only.
- Security and OTEL are in subpackages to reduce coupling and clarify responsibilities.
- Strategy manager sits in its own subpackage; the high‑level strategy logger remains in `service` to avoid import cycles.

### Build & Test

Run from module root (where `go.mod` lives):

```
go clean -cache -modcache
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build ./...
```

If you need CGO (macOS), first accept Xcode license:

```
sudo xcodebuild -license accept
sudo xcodebuild -runFirstLaunch
CGO_ENABLED=1 go build ./...
```

If you see `package ... is not in std`, ensure you are at the module root and clean caches as above.
ตารางที่ sink ใช้งาน (สร้างอัตโนมัติถ้าเปิด `auto_create`):
```sql
CREATE TABLE IF NOT EXISTS obs.logs (
  datetime   DateTime,
  level      LowCardinality(String),
  msg        String,
  trace_id   String,
  span_id    String,
  request_id String,
  user_id    String,
  module     String,
  tenant_id  String,
  error      String,
  fields     String
) ENGINE = MergeTree
ORDER BY (datetime, level);
```
ตัวอย่าง query ทั่วไป:
```sql
-- ล่าสุด 100 แถว
SELECT datetime, level, module, msg
FROM obs.logs
ORDER BY datetime DESC
LIMIT 100;

-- ปริมาณ log ตาม level ราย 1 นาที (ชั่วโมงล่าสุด)
SELECT toStartOfMinute(datetime) AS ts, level, count() AS cnt
FROM obs.logs
WHERE datetime >= now() - INTERVAL 1 HOUR
GROUP BY ts, level
ORDER BY ts;

-- Error rate ต่อ 1 นาที
SELECT
  toStartOfMinute(datetime) AS ts,
  countIf(level IN ('ERROR','FATAL')) AS errors,
  count() AS total,
  errors / total AS error_rate
FROM obs.logs
WHERE datetime >= now() - INTERVAL 1 HOUR
GROUP BY ts
ORDER BY ts;

-- กรองตาม module + user
SELECT *
FROM obs.logs
WHERE module = 'checkout' AND user_id = 'u-1001' AND level IN ('WARN','ERROR','FATAL')
ORDER BY datetime DESC
LIMIT 50;

-- ดึงฟิลด์ย่อยจาก JSON 'fields'
SELECT datetime,
  JSONExtractString(fields, 'order_id')  AS order_id,
  JSONExtract(fields, 'amount','Float64') AS amount
FROM obs.logs
WHERE JSONHas(fields, 'order_id')
ORDER BY datetime DESC
LIMIT 50;
```
ทดสอบอย่างรวดเร็วด้วย HTTP:
```bash
curl 'http://localhost:8123/?query=SELECT%20count()%20FROM%20obs.logs'
```

## Web (Gin) with LogTrc
Attach logger ต่อ request + ตัวอย่างใช้งาน ResponseBuilder + context แบบ type-safe
```go
// Middleware แนบ log ต่อ request (มี fields พื้นฐานให้)
r := gin.New()
r.Use(logtrc.Middleware("checkout"))

// Health check
r.GET("/health", func(c *gin.Context) {
    log := logtrc.GetLog(c)
    log.Info("health ok")
    c.JSON(200, gin.H{"ok": true})
})

// ตัวอย่างดึง/ส่ง order พร้อม response builder
r.GET("/orders/:id", func(c *gin.Context) {
    // ใส่ TraceID ลง context แบบ type-safe
    ctx := util.WithTraceID(c.Request.Context(), "trace-demo-001")
    log := logtrc.GetLog(c).Ctx(ctx).F("route","/orders/:id")

    id := c.Param("id")
    // ... ทำงาน fetch ...
    log.F("order_id", id).Info("fetched order")

    // ใช้ ResponseBuilder สร้าง response และพิมพ์ log สรุป
    logtrc.GetLogTrcFrmGin(c, "get_order").
      R(200, logtrc.Opts.Msg("ok"), logtrc.Opts.Body(gin.H{"order_id": id}), logtrc.Opts.Prt(true)).
      Send()
})

_ = r.Run(":8080")
```

### OpenTelemetry (optional)
```go
otel, _, _ := logtrc.NewOTelWithService("checkout", "1.0.0", "prod", "jaeger:4317", logtrc.INFO)
r := gin.New()
r.Use(logtrc.OTelMiddleware(otel))

r.GET("/", func(c *gin.Context) {
    log := logtrc.GetOTelLog(c)
    log.Info("otel request")
    c.String(200, "ok")
})
```

## Strategy Engine (filter/sample/mask)
```go
// via core/service facade
logbrut := service.NewStrategyLogBrt(service.INFO, logtrc.NewFastStdoutSink())
logbrut.AddFilter(service.NewLevelFilter(service.INFO, service.ERROR))
logbrut.AddSampler(service.NewRateSampler(0.25)) // 25%
logbrut.AddMasker(security.NewPIIMaskerStrategy())
logbrut.F("email", "john.doe@example.com").Info("created user")
```

### Strategy Examples (Detailed)

CLI toggles (perf_runner):

```bash
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull -strategy_no_mask=true
go run ./cmd/perf_runner -summary -total=150000 -workers=cpu -modes=strategy -sink=devnull -strategy_no_sample=true
```

Code toggles:

```go
s := service.NewStrategyLogBrt(service.INFO)
// Remove by kind/name (see strategies.go names)
s.RemoveStrategy("sampler", "rate_sampler")
s.RemoveStrategy("masker",  "regex_masker")
// Re-add/adjust at runtime
s.AddSampler(service.NewRateSampler(0.5))      // 50%
s.AddFilter(service.NewLevelFilter(service.INFO, service.ERROR))
// Masking strategy from security package (PII)
s.AddMasker(security.NewPIIMaskerStrategy())

// Typical: filter -> sample -> mask -> emit
s.F("email","john.doe@example.com").Info("user created")
```

## Async Pipeline
- `AsyncLogBrt` and `StrategyLogBrt` ล็อกแบบ non-blocking, มี worker batcher/flush
- เรียก `Stop()` เมื่อจบโปรเซสเพื่อให้ flush งานค้าง

## Security Mode
```go
sec, _ := logtrc.NewSecurityLogBrt("checkout", "1.0.0", "prod", "jaeger:4317", logtrc.INFO)
sec.F("email", "john.doe@example.com").Info("masked output")
```

## Response Builder (LogTrc)
```go
rb := logtrc.GetLogTrcFrmGin(c, "create_order").R(200, logtrc.Opts.Msg("ok"))
rb.Send()
```

## Tuning
- **Level**: ลดเป็น `INFO`/`WARN` ในโปรดักชัน
- **Async**: ปรับขนาด batch และ timeout (ดู `NewBufferedSinkWith`, `AsyncPipeline`)
- **Sampling**: ใช้ `RateSampler` หรือ `AdaptiveSampler` ลดปริมาณลอค
- **Masking**: เปิด `Masking(true)` เมื่อมี PII เพื่อความปลอดภัย
- **File sink**: ตั้งค่า rotation เพื่อจำกัดขนาดไฟล์

## Benchmarks
รัน benchmark พื้นฐาน:
```bash
go test ./benchmarks -bench .
```

รัน perf runner (ตัด IO):
```bash
go run ./cmd/perf_runner -total=300000 -workers=cpu -modes=unified,async,strategy -structured=true
```

## E2E with Docker Compose

Bring up the observability stack (Loki, Tempo, Jaeger, Prometheus, Grafana, Promtail, OTEL Collector, ClickHouse, Wiremock):

```bash
cd compose
docker compose up -d
```

Endpoints (defaults):

- OTLP gRPC: `localhost:4317`
- Prometheus scrape (from collector): `localhost:8889`
- Loki HTTP: `http://localhost:3100/loki/api/v1/push`
- Jaeger UI: `http://localhost:16686`
- Tempo Query: `http://localhost:3200`
- Prometheus UI: `http://localhost:9090`
- Grafana UI: `http://localhost:3000`
- ClickHouse HTTP: `http://localhost:8123`
- Wiremock (mock webhooks): `http://localhost:8089`

Smoke test examples (in separate shells):

```bash
# 1) OTEL + Loki example (falls back to async if OTEL not available)
go run ./examples/otel_loki

# 2) Full demo (file + loki + zerolog + security masking)
go run ./examples/full_demo

# 3) HTTP (Gin) with middleware; then GET http://localhost:8081/health
go run ./examples/http
```

### Promtail → Loki (tail local logs/)

1) Bring up the stack:
```bash
cd compose && docker compose up -d
```

2) Run the file-logging example (writes JSON lines to `./logs/app.log`):
```bash
go run ./examples/promtail_file
```

3) Open Grafana → Explore → Data source: Loki → Query:
```
{job="obs-brutal"}
```
You should see the JSON logs shipped via Promtail.

Notes:
- Promtail tails `../logs` (mounted to `/var/log/app`) with glob `/var/log/app/*.log` (see `compose/promtail-config.yml`).
- The example uses `NewLumberjackSink` + `NewBufferedWrap` for production‑like file logging.

### Minimal Stack (faster, lower resource)

Use only Loki + Grafana (no Promtail, no Tempo/Jaeger/Collector/Prometheus):

```bash
cd compose
docker compose --profile mini up -d  # starts loki + grafana with resource limits
```

Send logs directly to Loki (no Promtail) in your app:

```go
lk := logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{"app":"obs-brutal","env":"dev"})
log := logtrc.NewAsyncCfg(2000, 4, 100*time.Millisecond, logtrc.INFO, lk)
log.F("module","mini").Info("hello loki direct")
```

Open Grafana → Explore → Loki and query by `{app="obs-brutal"}`.

### Quickstart by profiles

- mini (Loki + Grafana only):
  ```bash
  cd compose
  docker compose --profile mini up -d
  # app side:
  # use Loki sink directly (see snippet above) and open Grafana at http://localhost:3000
  ```

- file (Promtail + Loki + Grafana):
  ```bash
  cd compose
  docker compose --profile file up -d
  # run example that writes to ./logs/
  go run ./examples/promtail_file
  # open Grafana -> Explore -> Loki -> query {job="obs-brutal"}
  ```

- full (all services):
  ```bash
  cd compose
  docker compose --profile full up -d
  # run any example (e.g., otel_loki) and browse Grafana/Prometheus/Jaeger/Tempo
  ```

Notes:

- To verify Prometheus metrics: `curl http://localhost:8889/metrics`
- To test Slack webhook sink, point webhook URL to Wiremock, e.g., `http://localhost:8089/notify`
- Promtail tails local `logs/` directory (mounted in compose); configure file sink to write under `logs/` to see logs in Loki via Promtail.

## Migration

- Global convenience removed: no more `logtrc.Debug/Info/Warn/Error/Fatal`, `With*`, or `WithContext` globals.
  - Migrate: create an instance and use fluent API.
    - `log := logtrc.NewDefault(); log.Info("msg")`
    - or `log := logtrc.New(logtrc.Async(true))`
- Strategy helpers removed from facade: no more `logtrc.CreateLevelFilter/RateSampler/AdaptiveSampler/PIIMasker`.
  - Migrate: use core services directly.
    - `service.NewLevelFilter(...)`, `service.NewRateSampler(...)`, `service.NewAdaptiveSampler(...)`, `security.NewPIIMaskerStrategy()`
- Buffered sink wrapper: prefer `logtrc.NewBufferedWrap(inner, size, timeout)` to batch any sink.
- Loki sink now supports batching/retry via `Configure` (keys: `batch_size`, `timeout_ms`, `max_retries`, `retry_base_ms`).
- Timestamps: JSON output uses `RFC3339Nano` for higher precision.
- Async DropOldest: fixed to truly drop the oldest when queue is full; counters now accurate.
- IDs promoted: `trace_id/span_id/request_id/user_id/module/tenant_id` are promoted to top-level JSON fields.

## Performance Tuning

Recommended defaults (start here and tune with metrics):

- Async + Buffered
  - `buffer_size`: 1000
  - `buffer_timeout`: 50–100ms
  - Adjust to keep `dropped≈0` and avoid sustained `queue_size` spikes.
- File output
  - Lumberjack + Buffered
  - Example: `filename=logs/app.log`, `rotate_size_bytes=10–50MB`, `max_backups=7–14`, `compress=true`
- Network (Loki/OTLP)
  - Async + batching/backoff
  - Queue capacity for peak load (≈ peak logs/sec × flush window)
  - Define clear drop policy & alerting
- Hot‑path hygiene
  - Avoid `fmt.Sprintf` in hot path; prefer structured fields
  - Limit number/size of fields when throughput matters
  - Put filters before samplers to short‑circuit early (e.g., drop DEBUG in prod)
- Scaling
  - Test workers `1`, `cpu`, `2cpu` on your workload; IO sinks often limit scaling

Example baseline (devnull, CPU=8, structured=false):

- Unified: ~7–8.5M logs/sec (≈0.12–0.16 µs/log)
- Async: ~7–7.6M logs/sec (≈0.13–0.20 µs/log)

With structured=true, throughput drops (JSON/fields overhead). With file/buffered, ~200–280k logs/sec is typical on a single host.

Monitor with OTEL (built‑in hooks in `OTelLogBrt` and `NewAsyncLogBrtWithTelemetry`):

- Counters (delta): `obs_async_processed_total`, `obs_async_dropped_total`, `obs_async_batches_total`
- Gauge‑like histogram: `obs_async_queue_size`

Alerting suggestions:

- Dropped logs: `obs_async_dropped_total` increases continuously for ≥ 1m
- Backlog: `obs_async_queue_size` ≥ 80% capacity for ≥ 1m
- Throughput anomaly: sudden drop in `processed` rate vs baseline (SLO‑based)

## Notes
- โค้ดใน `internal/core` คือ engine ภายใน; แนะนำให้ใช้งานผ่าน `logtrc` facade สำหรับ API ที่คงเสถียร
- OTEL เป็นทางเลือก (optional). หากไม่ได้ตั้งค่า endpoint จะไม่เปิดใช้งาน

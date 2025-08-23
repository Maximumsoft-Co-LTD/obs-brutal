## obs-brutal

High-performance logging and tracing toolkit for Go with clean APIs, optional OpenTelemetry, strategy-based filtering/sampling/masking, async pipeline, and web-friendly helpers.

### Highlights
- **Unified logger core**: fast structured logging with fluent chaining
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
    log.With("user_id", 123).Info("structured")
}
```

## Quick Setup (OTLP + Loki + Zerolog)
```go
package main

import (
    "obs-brutal/logtrc"
)

func main() {
    sinks := []logtrc.Sink{
        logtrc.NewConsoleSink(true), // toggle terminal on/off
        logtrc.NewOTLPSink("localhost:4317"),
        logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{
            "app": "obs-brutal", "env": "dev",
        }),
        logtrc.NewZerologSink(),
    }
    log := logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)
    log.F("module", "quicksetup").Info("obs-brutal ready")
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
```

### Multi-sinks (tee หลายปลายทางพร้อมกัน)
ส่งออกหลายช่องทางพร้อมกันได้ โดยใส่หลาย `Sink` ตอนสร้าง logger
```go
sinks := []logtrc.Sink{
  logtrc.NewConsoleSink(false),
  logtrc.NewLumberjackSink(),
  logtrc.NewOTLPSink("otel-collector:4317"),
  logtrc.NewLokiPushSink("http://loki:3100/loki/api/v1/push", map[string]string{"app":"svc","env":"prod"}),
  logtrc.NewClickHouseSink(),
}
log := logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)
log.F("module","demo").Info("multi-sinks tee")
```

## ClickHouse (SQL examples)
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
Attach a lightweight logger per request with common fields and a response helper:
```go
r := gin.New()
r.Use(logtrc.Middleware("checkout"))

r.GET("/", func(c *gin.Context) {
    log := logtrc.GetLog(c)
    log.Info("hello from web")
    c.JSON(200, gin.H{"ok": true})
})

_ = r.Run(":8080")
```

### OpenTelemetry (optional)
```go
otel, _ := logtrc.NewOTelLogBrt("checkout", "1.0.0", "prod", "jaeger:4317", logtrc.INFO)
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
// internal/core usage example
logger := core.NewStrategyLogBrt(core.INFO, logtrc.NewFastStdoutSink())
logger.AddFilter(core.NewLevelFilter(core.INFO, core.ERROR))
logger.AddSampler(core.NewRateSampler(0.25)) // 25%
logger.AddMasker(core.NewPIIMasker())        // enterprise masking

logger.F("email", "john.doe@example.com").Info("created user")
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

## Notes
- โค้ดใน `internal/core` คือ engine ภายใน; แนะนำให้ใช้งานผ่าน `logtrc` facade สำหรับ API ที่คงเสถียร
- OTEL เป็นทางเลือก (optional). หากไม่ได้ตั้งค่า endpoint จะไม่เปิดใช้งาน



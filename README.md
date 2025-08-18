## คู่มือใช้งาน github.com/Maximumsoft-Co-LTD/obs-brutal (Thai)

ระบบนี้เป็นชุดเครื่องมือ Observability/Logging ที่ออกแบบแบบพอร์ตแยก Inbound/Outbound พร้อมอะแดปเตอร์, มิดเดิลแวร์สำหรับ HTTP/Gin, การเก็บเมตริก Prometheus และการ Trace ด้วย OpenTelemetry

ด้านล่างคือคู่มือพร้อมตัวอย่างครบทุก interface และทุก handler ที่มีในโปรเจกต์นี้ เพื่อให้คุณนำไปใช้/ต่อยอดได้ทันที

### สารบัญสั้น

- ติดตั้งและนำเข้า
- Quick Start (Global logger, Context logger)
- Middleware (net/http, Gin, OTel, SafePrometheus)
- Inbound Adapters (HTTPAdapter, CLIAdapter, MiddlewareAdapter)
- Logger API ครบทุกเมธอด (Ctx, F, Fs, Err, TID/SID/UID/RID/IP/Sess/Tenant/Mod)
- Use cases/Service (LogWithContext, LogErr, LogStruct)
- Features/ErrCategories (ตัวอย่างครบ)
- Outbound Ports & Adapters (Sinks, Buffered/Multiplex, Formatters, Metrics, ConfigSrc, TraceSrc, HTTPClient, Cache)
- เครื่องมือ/ยูทิลิตี้ (ParseLevel, GetLoggerFromContext, GenerateRequestID)
- Best practices/ความปลอดภัย

### ติดตั้งและนำเข้าแพ็กเกจ

```go
import (
    obsv "github.com/Maximumsoft-Co-LTD/obs-brutal/pkg/logbrutal"
    inboundAdapter "github.com/Maximumsoft-Co-LTD/obs-brutal/pkg/logbrutal/adapters/inbound"
    outboundAdapter "github.com/Maximumsoft-Co-LTD/obs-brutal/pkg/logbrutal/adapters/outbound"
)
```

### Quick Start: สร้าง Logger และเขียน Log แบบง่าย

```go
logger, _ := obsv.NewLogger(
    obsv.WithLevel(obsv.InfoLevel),
    obsv.WithSinks(
        obsv.NewStdoutSink(),
    ),
)

logger.F("module", "demo").Info("hello world")

// ป้องกัน Panic ของ Logger ด้วย Safe Wrapper
safe := obsv.NewSafeLogger(logger)
safe.Info("safe logging ok")
```

### Sinks (Outbound) และตัวอย่างการใช้งาน

- สร้าง Sink พื้นฐานและใช้งาน Multiple/Buffered

```go
// Stdout
s1 := obsv.NewStdoutSink()

// File (รองรับ rotation)
s2 := obsv.NewFileSink("app.log", 50, 7, 3, true)

// HTTP Sink (batch)
s3 := obsv.NewHTTPSink("https://example.com/logs", map[string]string{"Authorization": "Bearer xxx"}, 100)

// Loki Sink
s4 := obsv.NewLokiSink("http://loki:3100", map[string]string{"app": "demo"}, 100)

// OTLP Sink (placeholder สำหรับ log; ใช้ health check ได้)
s5, _ := obsv.NewOTLPSink("localhost:4317", true)

// Buffered และ Multiplex
buf := obsv.NewBufferedSink(s1, 1000, time.Second)
multi := obsv.NewMultiplexSink(s2, s3, s4, s5, buf)

logger, _ := obsv.NewLogger(obsv.WithLevel(obsv.InfoLevel), obsv.WithSinks(multi))
logger.Info("write via multiplex+buffered")
```

### ใช้งาน Global Logger อย่างเร็ว

```go
obsv.Info("service started")
obsv.WithField("module", "billing").Error("payment failed")
```

### Formatters (Outbound)

```go
// ผ่าน API
jsonFmt := obsv.NewJSONFormatter()
textFmt := obsv.NewTextFormatter()
logfmt := obsv.NewLogfmtFormatter()

// ผูกกับ StdoutSink ที่ระดับอะแดปเตอร์โดยตรง (ตัวอย่างจาก adapters/outbound)
_ = outboundAdapter.NewStdoutSink(outboundAdapter.NewPrettyJSONFormatter())
```

หมายเหตุ: Formatter จะ scrub ข้อมูลอ่อนไหวอัตโนมัติ (password, token, secret, api_key ฯลฯ) จาก `entry.Fields` ระดับบนสุด

### Metrics (Prometheus) ตัวอย่างครบ

- แบบ OTel Exporter (มีใน `obsv.NewOTelProvider`) ใช้กับ `promhttp.Handler()` ภายใน
- แบบมิดเดิลแวร์ปลอดภัย `SafePrometheus` สำหรับ Gin

```go
// ใช้ SafePrometheus กับ Gin
r := gin.New()
p := obsv.NewSafePrometheusWith("http", "demo-svc", "prod", true)
p.Use(r)            // ติด middleware
p.SetMetricsPath(r) // สร้าง /metrics

// หรือรัน exporter แยกพอร์ต
p.SetListenAddress(":9090")
```

นอกจากนี้ยังมี outbound metrics providers:

```go
// PrometheusMetricsProvider (OTel metric API)
mp, _ := outboundAdapter.NewPrometheusMetricsProvider("demo-svc")
_ = mp.Log(context.Background(), obsv.InfoLevel, "demo", 0)
_ = mp.Err(context.Background(), "demo", "validation")

// SimpleMetricsProvider (in-memory สำหรับทดสอบ)
simpleMp := outboundAdapter.NewSimpleMetricsProvider()
_ = simpleMp.Log(context.Background(), obsv.InfoLevel, "demo", 0)
```

### Tracing (OpenTelemetry)

```go
// ใช้งานผ่าน Provider ระดับแพ็กเกจ obsv
provider, _ := obsv.NewOTelProvider("demo-svc", "localhost:4317", true)
defer provider.Shutdown(context.Background())

// เริ่ม span และเชื่อมกับ logger
ctx, span, logWithSpan := obsv.StartSpanWithLogger(context.Background(), logger, "DemoOperation")
defer span.End()
logWithSpan.Info("inside span")
```

### การใช้งานร่วมกับ Gin

```go
r := gin.New()

// มิดเดิลแวร์ Log และ OTEL
r.Use(obsv.GinMiddleware(logger))
provider, _ := obsv.NewOTelProvider("demo-svc", "localhost:4317", true)
r.Use(obsv.OTelGinMiddleware(logger, provider))

r.GET("/hello", func(c *gin.Context) {
    // ดึง Logger แบบง่ายจาก Gin + สร้าง span ให้ด้วย
    log := obsv.GetLogFrmGin(c, "HelloHandler")
    defer log.Close()

    log.F("user", "alice").Prt("say hello")

    // Response builder อย่างง่าย
    opts := obsv.OptsResponse().Msg("ok").Response(gin.H{"greet": "hello"})
    log.R(200, opts)
})

_ = r.Run(":8080")
```

### การใช้งานร่วมกับ net/http

```go
mux := http.NewServeMux()
mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
    logger.Ctx(r.Context()).Info("hello http")
    _, _ = w.Write([]byte("ok"))
})

// ต่อ middleware logging
wrapped := obsv.HTTPMiddleware(logger)(mux)

// ใส่ OTel + Logging พร้อมกัน
provider, _ := obsv.NewOTelProvider("demo-svc", "localhost:4317", true)
wrapped = obsv.OTelHTTPMiddleware(logger, provider, "demo-svc")(wrapped)

srv := &http.Server{Addr: ":8080", Handler: wrapped}
_ = srv.ListenAndServe()
```

### การดึง Logger จาก Context

```go
// ใน handler ของคุณ (net/http)
func handler(w http.ResponseWriter, r *http.Request) {
    if lg, ok := logbrutal.GetLoggerFromContext(r.Context()); ok {
        lg.Info("got logger from context")
    }
}
```

### Inbound Adapters: HTTPAdapter (ครบทุก Handler)

อะแดปเตอร์นี้เพิ่ม REST endpoints สำหรับการ log และ config

- เส้นทางทั้งหมดที่ลงทะเบียนโดย `HTTPAdapter.RegisterRoutes` ภายใต้ prefix `/api/v1/logging`
  - POST `/log`
  - POST `/error`
  - POST `/structured-error`
  - GET `/features`
  - POST `/features` (demo; not implemented)
  - GET `/features/:name`
  - GET `/error-categories`
  - POST `/error-categories` (demo; not implemented)
  - GET `/error-categories/:category`
  - GET `/health`

ตัวอย่างการใช้งาน:

```go
// สร้าง dependency: service, features, categories (ตัวอย่างง่าย)
var (
    configSrc outboundAdapter.RedisConfigProvider // หรืออื่น ๆ ที่ implements ConfigSrc
)

// สมมติคุณมี registry สำหรับ features/err-categories ตาม interface (ดูด้านล่าง)
usecase := usecases.NewLoggingUseCase(
    logger, featureRegistry, errCategoryRegistry, &configSrc, outboundAdapter.NewSimpleMetricsProvider(),
)

api := inboundAdapter.NewHTTPAdapter(usecase, featureRegistry, errCategoryRegistry)
r := gin.New()
api.RegisterRoutes(r)
_ = r.Run(":8080")
```

ตัวอย่างเรียกใช้งานด้วย curl:

```bash
curl -X POST http://localhost:8080/api/v1/logging/log \
  -H 'Content-Type: application/json' \
  -d '{"level":"info","message":"hello","fields":{"k":"v"}}'

curl -X POST http://localhost:8080/api/v1/logging/error \
  -H 'Content-Type: application/json' \
  -d '{"error":"boom","category":"validation"}'

curl -X POST http://localhost:8080/api/v1/logging/structured-error \
  -H 'Content-Type: application/json' \
  -d '{"code":"E100","message":"db fail","category":"database","details":{"q":"select 1"}}'
```

### Inbound Adapters: CLIAdapter (ครบทุกคำสั่ง)

```go
cli := inboundAdapter.NewCLIAdapter(usecase, featureRegistry, errCategoryRegistry)
root := cli.Root()
_ = root.Execute()

// คำสั่งที่มี:
// - logbrutal log -m "hello" -l info -f k=v --module api --tenant t1
// - logbrutal feature list|get <name>
// - logbrutal category list|get <category> | category error <category> <message>
// - logbrutal config show|validate <file>
```

### Inbound Adapters: MiddlewareAdapter (Gin)

```go
m := inboundAdapter.NewMiddlewareAdapter(logger)
r := gin.New()
r.Use(m.GinLoggingMiddleware())
r.Use(m.GinErrorHandlingMiddleware())
```

### Logger API ครบทุกเมธอด (ตัวอย่างสั้นๆ)

```go
lg, _ := obsv.NewLogger(obsv.WithLevel(obsv.DebugLevel))
lg = lg.Ctx(context.Background()).
    F("k", "v").Fs(map[string]any{"x":1}).
    TID("trace-1").SID("span-1").UID("user-1").RID("req-1").
    IP("127.0.0.1").Sess("sess-1").Tenant("tenant-1").Mod("orders")

lg.Debug("debug msg")
lg.Info("info msg")
lg.Warn("warn msg")
lg.Err(fmt.Errorf("boom")).Error("error msg")
lg.Fatal("fatal msg")

// ระดับ log
lg.Level(obsv.WarnLevel)
cur := lg.GetLevel()

// ตัวนับ
_ = lg.Logged()
_ = lg.Filtered()
```

### Use Case / Service (LogWithContext, LogErr, LogStruct)

```go
// สร้าง registry อย่างง่าย (ตัวอย่างเทสควรมี implementation จริงจังในโปรเจ็กต์)
featureRegistry := NewInMemoryFeatureRegistry() // สมมติ
errRegistry := NewInMemoryErrCategoryRegistry() // สมมติ

cfgSrc, _ := outboundAdapter.NewRedisConfigProvider("127.0.0.1:6379", "", 0, "obsv:")
metrics := outboundAdapter.NewSimpleMetricsProvider()

uc := usecases.NewLoggingUseCase(lg, featureRegistry, errRegistry, cfgSrc, metrics)

ctx := context.Background()
_ = uc.LogWithContext(ctx, obsv.InfoLevel, "hello", map[string]any{"k":"v"})
_ = uc.LogErr(ctx, fmt.Errorf("boom"), "validation", map[string]any{"field":"email"})
_ = uc.LogStruct(ctx, domain.StructuredError{Code:"E100", Message:"db error", Category:"database"})
```

### Core Middlewares (ภายใต้ `pkg/logbrutal/middleware.go`)

- **HTTPMiddleware(logger)**: net/http middleware สำหรับ log
- **GinMiddleware(logger)**: Gin middleware สำหรับ log
- **OTelHTTPMiddleware(logger, provider, serviceName)**: net/http + OTel
- **OTelGinMiddleware(logger, provider)**: Gin + OTel

ดูตัวอย่างการใช้งานในส่วน Gin และ net/http ด้านบน

---

## Catalog: รายการ Interfaces ทั้งหมดและตัวอย่าง (Minimal)

ด้านล่างคือตัวอย่างโค้ดสั้น ๆ สำหรับแต่ละ interface/handler เพื่ออ้างอิงอย่างรวดเร็ว

### Inbound Ports (ใน `core/ports/inbound/logger.go`)

- **Logger**: ใช้งานผ่าน `obsv.NewLogger`

```go
logger, _ := obsv.NewLogger()
logger.Ctx(ctx).TID("t").SID("s").UID("u").RID("r").IP("1.2.3.4").Tenant("tnt").Mod("order").
    Fs(map[string]any{"k":"v"}).Err(fmt.Errorf("oops")).Warn("warn msg")
```

- **Simple**: อินเตอร์เฟสดีไซน์ไว้สำหรับ fluent API และ Response builder

ตัวอย่างการใช้งานสไตล์เดียวกันมีใน `simple.go` ผ่าน `GinLogger` (ดูด้านล่าง)

- **Trace**: ใช้สไตล์เดียวกับ `simple.Tracer` ใน `simple.go`

- **Response**: ใช้แนวคิดเดียวกับ `SimpleResponseBuilder` ใน `simple.go`

- **Service**: ยูสเคสหลักสำหรับ Log/Err/Struct

```go
// ตัวอย่างประกอบยูสเคส
uc := usecases.NewLoggingUseCase(logger, featureRegistry, errCategoryRegistry, configSrc, metrics)
_ = uc.LogWithContext(ctx, obsv.InfoLevel, "hello", map[string]any{"k":"v"})
_ = uc.LogErr(ctx, fmt.Errorf("boom"), "validation", nil)
_ = uc.LogStruct(ctx, domain.StructuredError{Code: "E100", Message: "fail", Category: "db"})
```

- **Features / Feature**: ลงทะเบียนและ Apply ฟีเจอร์กับ Logger

```go
type MyFeature struct{}
func (f *MyFeature) Name() string { return "my_feature" }
func (f *MyFeature) Apply(l inbound.Logger) inbound.Logger { return l.F("feature", "on") }
func (f *MyFeature) Configure(map[string]any) error { return nil }

_ = featureRegistry.Register("my_feature", &MyFeature{})
log2 := featureRegistry.Apply(logger, []string{"my_feature"})
log2.Info("feature applied")
```

- **ErrCategories / ErrHandler**: จัดกลุ่ม error และกำหนดพฤติกรรม

```go
type ValidationHandler struct{}
func (h *ValidationHandler) Category() string { return "validation" }
func (h *ValidationHandler) Handle(l inbound.Logger, err error, details map[string]any) {
    l.Err(err).Warn("Validation error")
}
func (h *ValidationHandler) ShouldAlert() bool      { return false }
func (h *ValidationHandler) Severity() domain.Level { return domain.WarnLevel }

_ = errCategoryRegistry.Register("validation", &ValidationHandler{})
```

- **Config (โครงสร้างตั้งค่า)**

```go
cfg := inbound.Config{Level: obsv.InfoLevel, Svc: "svc", Env: "prod", Sinks: []string{"stdout"}}
```

### Outbound Ports (ใน `core/ports/outbound/infrastructure.go`)

- **Sink**: สร้าง Sink เอง

```go
type MySink struct{}
func (s *MySink) Write(e *domain.LogEntry) error { fmt.Println("LOG:", e.Message); return nil }
func (s *MySink) Close() error                   { return nil }
func (s *MySink) Name() string                   { return "my_sink" }
func (s *MySink) Health() error                  { return nil }
```

- **ConfigSrc**: ใช้ Redis adapter ที่มีให้

```go
cfgSrc, _ := outboundAdapter.NewRedisConfigProvider("127.0.0.1:6379", "", 0, "obsv:")
level := cfgSrc.Level("order", "tenantA")
_ = cfgSrc.Sub(func(){ fmt.Println("config changed") })
```

- **Metrics**: ดูตัวอย่างด้านบน (PrometheusMetricsProvider, SimpleMetricsProvider)

- **Alerter**: ตัวอย่างตัวส่งแจ้งเตือนแบบกำหนดเอง

```go
type ConsoleAlerter struct{}
func (a *ConsoleAlerter) Send(ctx context.Context, alert outbound.Alert) error {
    fmt.Printf("[ALERT] %s: %s\n", alert.Level.String(), alert.Msg)
    return nil
}
func (a *ConsoleAlerter) ShouldSend(ctx context.Context, alert outbound.Alert) bool { return true }
func (a *ConsoleAlerter) Health() error  { return nil }
func (a *ConsoleAlerter) Close() error   { return nil }
```

- **TraceSrc / Span / Tracer**: ใช้ OTelTraceProvider

```go
tp, _ := outboundAdapter.NewOTelTraceProvider("demo-svc", "localhost:4317", true)
ctx, span := tp.StartSpan(context.Background(), "work")
span.Attr("k", "v"); span.End()
```

- **LogStorage**: โครงสร้างตัวอย่าง (ยังไม่มีอะแดปเตอร์ในโปรเจกต์นี้)

```go
type MemoryLogStore struct{ items []domain.LogEntry }
func (m *MemoryLogStore) Store(ctx context.Context, e *domain.LogEntry) error { m.items = append(m.items, *e); return nil }
func (m *MemoryLogStore) Query(ctx context.Context, q outbound.LogQuery) ([]domain.LogEntry, error) { return m.items, nil }
func (m *MemoryLogStore) DelOld(ctx context.Context, before time.Time) error { return nil }
func (m *MemoryLogStore) Health() error { return nil }
func (m *MemoryLogStore) Close() error  { return nil }
```

- **Filter / Sampler / Formatter**: ตัวอย่าง Formatter มีให้ครบแล้วใน `adapters/outbound/formatters.go`

```go
type PassAllFilter struct{}
func (f *PassAllFilter) Apply(e *domain.LogEntry) *domain.LogEntry { return e }
func (f *PassAllFilter) Name() string                              { return "pass_all" }
func (f *PassAllFilter) Configure(map[string]any) error            { return nil }

type RateSampler struct{ Rate float64 }
func (s *RateSampler) Sample(e *domain.LogEntry) bool { return rand.Float64() < s.Rate }
func (s *RateSampler) Name() string                   { return "rate" }
func (s *RateSampler) Configure(cfg map[string]any) error { if v,ok:=cfg["rate"].(float64); ok { s.Rate=v }; return nil }
```

- **Factory / SinkFactory**: โครงตัวอย่าง

```go
type MyFactory struct{}
func (f *MyFactory) Filter(name string, cfg map[string]any) (outbound.Filter, error)   { return &PassAllFilter{}, nil }
func (f *MyFactory) Sampler(name string, cfg map[string]any) (outbound.Sampler, error) { return &RateSampler{Rate:0.5}, nil }
func (f *MyFactory) Formatter(name string, cfg map[string]any) (outbound.Formatter, error) { return outboundAdapter.NewJSONFormatter(), nil }
func (f *MyFactory) RegFilter(name string, c outbound.FilterCreator)         {}
func (f *MyFactory) RegSampler(name string, c outbound.SamplerCreator)       {}
func (f *MyFactory) RegFormatter(name string, c outbound.FormatterCreator)   {}

type MySinkFactory struct{}
func (f *MySinkFactory) Create(cfg outbound.SinkConfig) (outbound.Sink, error) { return obsv.NewStdoutSink(), nil }
func (f *MySinkFactory) Reg(name string, c outbound.SinkCreator)               {}
func (f *MySinkFactory) List() []string                                        { return []string{"stdout"} }
```

- **HTTPClient**: ใช้ค่าเริ่มต้น หรือทำเอง

```go
client := outboundAdapter.NewDefaultHTTPClient()
resp, _ := client.Get(context.Background(), "https://example.com/health", nil)
_ = resp
```

- **Queue / Cache**: มี Redis cache ให้พร้อมใช้งาน; Queue เป็นสัญญา (interface) สำหรับต่อยอด ตัวอย่าง Queue แบบ in-memory

```go
cache, _ := outboundAdapter.NewRedisCacheClient("127.0.0.1:6379", "", 0)
_ = cache.Set(context.Background(), "k", []byte("v"), time.Minute)
```

ตัวอย่าง Queue ง่าย ๆ:

```go
type InMemQueue struct{ subs []func([]byte, map[string]string) }
func (q *InMemQueue) Pub(ctx context.Context, queue string, msg []byte, hdr map[string]string) error {
    for _, h := range q.subs { h(msg, hdr) }
    return nil
}
func (q *InMemQueue) Sub(ctx context.Context, queue string, h func([]byte, map[string]string)) error {
    q.subs = append(q.subs, h); return nil
}
func (q *InMemQueue) Close() error { return nil }
```

### Simple API (ใน `pkg/logbrutal/simple.go`)

- **GinLogger** และ **Tracer** สำหรับงานกับ Gin ที่กระชับ

```go
// ภายใน handler
log := obsv.GetLogFrmGin(c, "OpName")
defer log.Close()

log.F("step", 1).Prt("start")
pr := log.FlatPr("db")
pr.Add(pr.Str("query", "select 1"))
pr.End()

// ส่ง response
opts := obsv.OptsResponse().Msg("ok").Response(gin.H{"id": 1})
log.R(200, opts)
```

---

## HTTP/Gin Handlers ที่มีให้ (สรุป):

- `HTTPAdapter.RegisterRoutes` (Gin): ดูเส้นทางด้านบน พร้อม handler ต่อไปนี้:
  - `handleLog`, `handleLogErr`, `handleLogStruct`, `listFeatures`, `registerFeature`, `getFeature`, `listErrorCategories`, `registerErrorCategory`, `getErrorCategory`, `healthCheck`

- `MiddlewareAdapter` (Gin):
  - `GinLoggingMiddleware`, `GinErrorHandlingMiddleware`, `GinTracingMiddleware`

- Core Middlewares (แพ็กเกจ `pkg/logbrutal`):
  - `HTTPMiddleware`, `GinMiddleware`, `OTelHTTPMiddleware`, `OTelGinMiddleware`

- Prometheus:
  - `SafePrometheus.HandlerFunc()` และ `prometheusHandler()` สำหรับ Gin

---

## ตัวอย่างไฟล์ config (อ้างอิง `config.yaml`)

```yaml
app:
  name: demo-svc
  env: prod

logging:
  level: INFO
  sinks:
    - stdout
  format: json
```

---

## หมายเหตุและแนวทางต่อยอด
- ความปลอดภัย/ความเป็นส่วนตัวของข้อมูล
  - ระบบจะ scrub คีย์อ่อนไหว (password/token/secret/api_key ฯลฯ) ใน `Fields` อัตโนมัติ
  - มิดเดิลแวร์จะไม่ log body แบบ `multipart/form-data` โดยค่าเริ่มต้น และจำกัดขนาด body ที่อ่าน
  - แนะนำให้ทำ allowlist/denylist เส้นทาง/คีย์สำหรับการ log body เพิ่มตามนโยบายของทีม

- การใช้ Context และ Logger ในแอปจริง
  - ใช้ `GetLoggerFromContext(ctx)` เพื่อดึง logger ที่ผูกกับ request ในส่วนลึกของโค้ด
  - มีการใส่ logger ลง context ทั้งแบบ typed และ string key เพื่อความเข้ากันได้

- `HTTPSink`
  - มี retry แบบ exponential backoff และแบ่ง batch อัตโนมัติเมื่อ payload ใหญ่ (~1MB)

- `BufferedSink`
  - หาก buffer เต็มจะ drop พร้อมนับจำนวน (atomic) และเตือนทาง stderr

- Redis Config
  - แนะนำเปิด `notify-keyspace-events` บน Redis เพื่อให้ subscribe การเปลี่ยนแปลงคอนฟิกทำงานได้เต็มที่ มิฉะนั้นระบบจะ fallback เป็นการโหลดเป็นระยะ (ควรปรับใช้ตามสภาพแวดล้อม)

- เวอร์ชัน Go
  - โปรเจ็กต์นี้ทดสอบกับ Go >= 1.21 (แนะนำใช้ toolchain เวอร์ชันเดียวกันใน CI/CD)

---

## ชุดตัวอย่างเพิ่มเติมในโค้ด (อ้างอิงโฟลเดอร์ `examples/`)

- `examples/http-complete/`: รวม middleware, OTel, Prometheus, HTTPAdapter
- `examples/features/`: สาธิตการใช้ Feature และ ErrCategories
- `examples/trace/`: สาธิตการใช้งาน tracing
- `examples/simple/`: ตัวอย่างเริ่มต้นอย่างย่อ
- `examples/logger/`: ตัวอย่างการใช้ logger ตรงๆ


- บาง interface เช่น `LogStorage`, `Queue`, `Factory` ในโปรเจกต์นี้เป็นสัญญาเพื่อให้คุณต่อยอด โดยตัวอย่างโค้ดในคู่มือแสดง skeleton ที่สามารถนำไป implement ได้ทันที
- สำหรับ `ErrCategories` และ `Features` คุณสามารถสร้าง registry แบบ in-memory ง่าย ๆ ตาม interface แล้วส่งเข้า `usecases.NewLoggingUseCase`
- OTLP sink ฝั่ง log ยังเป็น placeholder เพื่อความเข้ากันได้กับ ecosystem OTel ในอนาคต (ใช้งาน Health/การตั้งค่า endpoint ได้)

# github.com/Maximumsoft-Co-LTD/obs-brutal

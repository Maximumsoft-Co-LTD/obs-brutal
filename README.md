# obs-brutal - ระบบ Logging และ Observability สำหรับ Go

obs-brutal เป็น logging library ที่ออกแบบตาม Hexagonal Architecture พร้อม features ครบครัน สำหรับ microservices และ distributed systems

## 🚀 การติดตั้ง

### จาก Local Path
```bash
go mod init your-project
echo 'replace github.com/Maximumsoft-Co-LTD/obs-brutal => /path/to/obs-brutal' >> go.mod
go mod tidy
```

### จาก Git Repository
```bash
go get obs-brutal.yourdomain.com
```

## 🎯 การเริ่มต้นใช้งาน

### ตัวอย่างพื้นฐาน

```go
package main

import (
    "fmt"
    "net/http"
    "time"
    
    obsv "obs-brutal.yourdomain.com/logbrutal"
    "github.com/gin-gonic/gin"
)

func main() {
    // 1) สร้าง Logger พร้อม Sink
    stdout := obsv.NewStdoutSink()
    logger, _ := obsv.NewLogger(obsv.WithLevel(obsv.InfoLevel), obsv.WithSinks(stdout))
    
    // 2) สร้าง Gin Engine พร้อม Middleware
    r := obsv.NewGinEngine()
    r.Use(obsv.GinMiddleware(logger))
    
    // 3) เพิ่ม Routes
    r.GET("/ping", func(c *gin.Context) {
        lg := obsv.GetLogFrmGin(c, "PingHandler")
        defer lg.Close()
        
        // สร้าง Trace
        tracer := lg.FlatPr("ping.operation")
        defer tracer.End()
        
        // เพิ่ม Attributes
        tracer.Add(
            tracer.Str("endpoint", "/ping"),
            tracer.Str("method", c.Request.Method),
        )
        
        // Log Request
        lg.F("endpoint", "/ping").Prt("Ping request received")
        
        // Response
        lg.R(http.StatusOK)
        c.JSON(http.StatusOK, gin.H{"ok": "pong"})
    })
    
    r.Run(":8080")
}
```

## 📖 คู่มือการใช้งาน

### 1. Logger พื้นฐาน

#### สร้าง Logger
```go
// Logger แบบง่าย
logger, err := obsv.NewLogger(
    obsv.WithLevel(obsv.InfoLevel),
    obsv.WithSinks(obsv.NewStdoutSink()),
)

// Logger พร้อม Multiple Sinks
logger, err := obsv.NewLogger(
    obsv.WithLevel(obsv.InfoLevel),
    obsv.WithSinks(
        obsv.NewStdoutSink(),
        obsv.NewFileSink("app.log", 100, 30, 10, true),
    ),
)
```

#### การ Log พื้นฐาน
```go
// Log levels
logger.Debug("Debug message")
logger.Info("Info message")
logger.Warn("Warning message")
logger.Error("Error message")

// Log พร้อม Fields
logger.F("user_id", "123").Info("User logged in")
logger.Fs(map[string]interface{}{
    "user_id": "123",
    "action":  "login",
}).Info("User action")

// Log พร้อม Error
logger.Err(err).Error("Something went wrong")
```

### 2. Gin Integration

#### การติดตั้ง Middleware
```go
r := obsv.NewGinEngine()
r.Use(obsv.GinMiddleware(logger))

r.GET("/users/:id", func(c *gin.Context) {
    lg := obsv.GetLogFrmGin(c, "GetUser")
    defer lg.Close()
    
    userID := c.Param("id")
    lg.F("user_id", userID).Prt("Getting user")
    
    // ทำงานต่อ...
})
```

#### Response Logging
```go
r.POST("/users", func(c *gin.Context) {
    lg := obsv.GetLogFrmGin(c, "CreateUser")
    defer lg.Close()
    
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        lg.Err(err)
        lg.R(400)
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    // สำเร็จ
    lg.R(201)
    c.JSON(201, user)
})
```

### 3. Tracing

#### สร้าง Traces
```go
func handleRequest(c *gin.Context) {
    lg := obsv.GetLogFrmGin(c, "HandleRequest")
    defer lg.Close()
    
    // Parent Trace
    parentTrace := lg.FlatPr("request.process")
    defer parentTrace.End()
    
    // Child Trace
    childTrace := parentTrace.FlatPr("database.query")
    childTrace.Add(
        childTrace.Str("table", "users"),
        childTrace.Str("operation", "SELECT"),
        childTrace.Num("user_id", 123),
    )
    childTrace.End()
    
    // Error Trace
    if err != nil {
        errorTrace := parentTrace.FlatPr("error.handle")
        errorTrace.Err(err)
        errorTrace.End()
    }
}
```

#### Trace Attributes
```go
tracer := lg.FlatPr("operation")
defer tracer.End()

// เพิ่ม Attributes
tracer.Add(
    tracer.Str("service", "api"),
    tracer.Bool("success", true),
    tracer.Num("duration_ms", 125.5),
    tracer.Code(200),
    tracer.Detail("Processing completed"),
    tracer.Msg("Operation successful"),
)

// Complex Object
bodyData := map[string]interface{}{
    "user": map[string]interface{}{
        "id": "123",
        "name": "John",
    },
}
bodyAttrs := tracer.Body("request", bodyData)
tracer.Add(bodyAttrs...)
```

### 4. Sinks (การส่งออก Log)

#### Stdout Sink
```go
stdout := obsv.NewStdoutSink()
logger, _ := obsv.NewLogger(
    obsv.WithLevel(obsv.InfoLevel),
    obsv.WithSinks(stdout),
)
```

#### File Sink
```go
fileSink := obsv.NewFileSink(
    "app.log",  // ชื่อไฟล์
    100,        // ขนาดสูงสุด (MB)
    30,         // จำนวนไฟล์เก็บ
    10,         // วันเก็บ
    true,       // compress
)
```

#### HTTP Sink
```go
httpSink, err := obsv.NewHTTPSink(
    "https://logs.example.com/api/logs",
    map[string]string{"Authorization": "Bearer token"},
    100,  // batch size
    true, // retry
)
```

#### Loki Sink
```go
lokiSink := obsv.NewLokiSink(
    "http://localhost:3100/loki/api/v1/push",
    map[string]string{
        "service": "my-app",
        "env":     "production",
    },
    100, // batch size
)
```

#### Multiple Sinks
```go
multiplexSink := obsv.NewMultiplexSink(
    obsv.NewStdoutSink(),
    fileSink,
    httpSink,
    lokiSink,
)

logger, _ := obsv.NewLogger(
    obsv.WithLevel(obsv.InfoLevel),
    obsv.WithSinks(multiplexSink),
)
```

### 5. Formatters

```go
// JSON Formatter
jsonFormatter := obsv.NewJSONFormatter()
sink := obsv.NewStdoutSink()
sink.SetFormatter(jsonFormatter)

// Text Formatter
textFormatter := obsv.NewTextFormatter()

// Logfmt Formatter
logfmtFormatter := obsv.NewLogfmtFormatter()

// CEF Formatter (for security logs)
cefFormatter := obsv.NewCEFFormatter()
```

### 6. Configuration Management

#### จาก File
```yaml
# config.yaml
log:
  level: "info"
  file: "app.log"

otel:
  enabled: true
  service_name: "my-service"
  endpoint: "http://localhost:4318/v1/traces"
```

```go
cfg, err := config.LoadConfig("config.yaml")
```

#### Redis Config Provider
```go
configSrc, err := obsv.NewRedisConfigProvider(
    "localhost:6379", // address
    "",               // password
    0,                // database
    "myapp:",         // key prefix
)

// อ่าน config
level, err := configSrc.GetString("log.level")
```

### 7. Error Handling

#### Error Categories
```go
type ValidationErrorHandler struct{}

func (h *ValidationErrorHandler) Category() string { return "validation" }
func (h *ValidationErrorHandler) Severity() obsv.Level { return obsv.WarnLevel }
func (h *ValidationErrorHandler) ShouldAlert() bool { return false }
func (h *ValidationErrorHandler) Handle(logger obsv.Logger, err error, details map[string]interface{}) {
    logger.F("category", "validation").
           Fs(details).
           Err(err).
           Warn("Validation error occurred")
}

// ใช้งาน
errCategories := obsv.NewErrorCategories()
errCategories.Register("validation", &ValidationErrorHandler{})

errCategories.Handle(logger, err, "validation", map[string]interface{}{
    "field": "email",
    "value": "invalid-email",
})
```

### 8. Testing

#### Mock Logger
```go
func TestUserService(t *testing.T) {
    mockLogger := obsv.NewMockLogger()
    userService := NewUserService(mockLogger)
    
    err := userService.CreateUser(&User{Name: "John"})
    
    assert.NoError(t, err)
    assert.Greater(t, mockLogger.Logged(), 0)
}
```

#### Safe Logger
```go
var nilLogger obsv.Logger
safeLogger := obsv.NewSafeLogger(nilLogger)

// จะไม่ panic แม้ว่า underlying logger จะเป็น nil
safeLogger.Info("This won't panic")
```

## 🛠️ Advanced Features

### Buffered Sink
```go
// สำหรับ performance ที่ดีขึ้น
bufferedSink := obsv.NewBufferedSink(
    httpSink,
    1000,                    // buffer size
    100*time.Millisecond,    // flush interval
)
```

### Struct Tags และ PII Masking
```go
type User struct {
    ID       string `json:"id" log:"user_id"`
    Name     string `json:"name" log:"user_name"`
    Email    string `json:"email" log:"email" pii:"true"`
    Password string `json:"password" log:"-" pii:"true"`
}

user := User{
    ID:       "123",
    Name:     "John",
    Email:    "john@example.com",
    Password: "secret",
}

// ใช้ struct tags
fields := obsv.ExtractFields(user)
logger.Fs(fields).Info("User created")
// PII จะถูก mask อัตโนมัติ
```

### Context Propagation
```go
logger = logger.
    UID("user-123").
    TID("trace-456").
    RID("req-abc").
    IP("192.168.1.1").
    Sess("session-xyz").
    Tenant("tenant-1").
    Mod("user-service")
```

## 🚀 Production Best Practices

### Performance Optimization
```go
func productionLogger() obsv.Logger {
    // ใช้ buffered sinks สำหรับ remote endpoints
    lokiSink := obsv.NewLokiSink("http://loki:3100/loki/api/v1/push", 
        map[string]string{"service": "api"}, 100)
    bufferedLoki := obsv.NewBufferedSink(lokiSink, 1000, 500*time.Millisecond)
    
    // Local file สำหรับ immediate access
    fileSink := obsv.NewFileSink("app.log", 100, 30, 7, true)
    
    multiplexSink := obsv.NewMultiplexSink(bufferedLoki, fileSink)
    
    logger, _ := obsv.NewLogger(
        obsv.WithLevel(obsv.InfoLevel),
        obsv.WithSinks(multiplexSink),
    )
    
    return logger
}
```

### Monitoring Setup
```go
func setupMonitoring(router *gin.Engine) {
    // Health check
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status": "healthy",
            "timestamp": time.Now().Unix(),
        })
    })
    
    // Readiness check
    router.GET("/ready", func(c *gin.Context) {
        if !checkDatabase() || !checkRedis() {
            c.JSON(503, gin.H{"status": "not ready"})
            return
        }
        c.JSON(200, gin.H{"status": "ready"})
    })
}
```

## 📊 ตัวอย่างการใช้งานจริง

### Complete Example
ดูตัวอย่างที่สมบูรณ์ใน `_cmd/main.go`:

```bash
# รันตัวอย่าง
go run _cmd/main.go

# ทดสอบ endpoints
curl http://localhost:8080/ping      # Ping endpoint
curl http://localhost:8080/health    # Health check
curl http://localhost:8080/error     # Error demo
```

### Log Output
```json
{
  "client_ip": "::1",
  "endpoint": "/ping",
  "level": "INFO",
  "message": "Ping request received",
  "method": "GET",
  "path": "/ping",
  "request_id": "req-1755479186280259000",
  "timestamp": "2025-08-18T08:06:26+07:00",
  "user_agent": "curl/8.7.1"
}
```

## 🔧 Configuration Examples

### Environment Variables
```bash
export LOG_LEVEL=debug
export LOG_FILE=/var/log/app.log
export OTEL_ENABLED=true
export OTEL_ENDPOINT=http://jaeger:14268/api/traces
```

### Docker Compose
```yaml
version: '3.8'
services:
  app:
    build: .
    environment:
      - LOG_LEVEL=info
      - LOKI_URL=http://loki:3100/loki/api/v1/push
    volumes:
      - ./logs:/app/logs
```

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📝 License

This project is licensed under the MIT License.

## 🙏 Acknowledgments

- [OpenTelemetry](https://opentelemetry.io/) for tracing standards
- [Gin](https://gin-gonic.com/) for HTTP framework
- [Zap](https://github.com/uber-go/zap) for high-performance logging

---

สำหรับคำถามเพิ่มเติม กรุณาสร้าง [GitHub Issue](https://github.com/Maximumsoft-Co-LTD/obs-brutal/issues)

package main

import (
    "context"
    "flag"
    "net/http"
    "time"

    "obs-brutal/internal/util"
    "obs-brutal/logtrc"

    "github.com/gin-gonic/gin"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
    // Flags
    var (
        svc   = flag.String("svc", "demo-all", "service name")
        env   = flag.String("env", "dev", "environment")
        otel  = flag.String("otel", "localhost:4317", "OTLP gRPC endpoint")
        chOn  = flag.Bool("ch", true, "enable ClickHouse sink")
        file  = flag.Bool("file", false, "enable file sink (logs/app.log)")
        loki  = flag.Bool("loki", false, "enable Loki sink")
        httpOn= flag.Bool("http", true, "start Gin HTTP demo")
        mongoOn=flag.Bool("mongo", false, "enable Mongo demo routes")
        mongoURI=flag.String("mongo_uri", "mongodb://127.0.0.1:27017", "MongoDB URI")
        port  = flag.String("port", ":8085", "http listen addr")
    )
    flag.Parse()

    // Build sinks
    var sinks []logtrc.Sink
    sinks = append(sinks, logtrc.NewConsoleSink(true))
    if *file {
        f := logtrc.NewLumberjackSink()
        _ = f.Configure(map[string]interface{}{"filename":"logs/app.log","max_size_mb":50,"max_backups":7,"max_age_days":14,"compress":true})
        sinks = append(sinks, logtrc.NewBufferedWrap(f, 1000, 100*time.Millisecond))
    }
    if *loki {
        sinks = append(sinks, logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{"app": *svc, "env": *env}))
    }
    if *chOn {
        ch := logtrc.NewClickHouseSink()
        _ = ch.Configure(map[string]interface{}{"endpoint":"http://localhost:8123","database":"obs","table":"logs","auto_create":true})
        sinks = append(sinks, ch)
    }

    // Try OTel; fallback to async
    var log logtrc.LogBrt
    if ot, _, err := logtrc.NewOTelWithService(*svc, "1.0.0", *env, *otel, logtrc.INFO, sinks...); err == nil && ot != nil {
        log = ot
    } else {
        log = logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)
    }

    // Seed demo logs
    log.F("module","boot").F("env", *env).Info("demo-all started")
    for i := 0; i < 3; i++ { log.F("module","seed").F("i", i).Info("hello"); time.Sleep(50*time.Millisecond) }

    if !*httpOn {
        // No HTTP; emit a trace-like flow and exit after a moment
        ctx, demo := withSpanIf(log, context.Background(), "demo_span")
        demo.Ctx(ctx).F("module","trace").Info("span work")
        time.Sleep(200 * time.Millisecond)
        return
    }

    // HTTP server + optional Mongo routes
    r := gin.New()
    r.Use(logtrc.Middleware(*svc))

    r.GET("/health", func(c *gin.Context) {
        logtrc.GetLog(c).F("env", *env).Info("health")
        c.JSON(200, gin.H{"ok": true})
    })

    if *mongoOn {
        // Real Mongo demo
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        cli, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
        if err == nil {
            coll := cli.Database("demo").Collection("items")
            r.GET("/mongo", func(c *gin.Context) {
                mctx := util.WithTraceID(c.Request.Context(), "mongo-trace-001")
                lg := logtrc.GetLog(c).Ctx(mctx)
                cur, err := coll.Find(mctx, bson.M{"ok": true}, options.Find().SetLimit(5))
                if err != nil { lg.WithError(err).Error("mongo find"); c.JSON(500, gin.H{"error": err.Error()}); return }
                defer cur.Close(mctx)
                var out []bson.M; _ = cur.All(mctx, &out)
                lg.F("count", len(out)).Info("mongo fetched")
                c.JSON(200, out)
            })
            r.POST("/mongo", func(c *gin.Context) {
                mctx := util.WithTraceID(c.Request.Context(), "mongo-trace-001")
                lg := logtrc.GetLog(c).Ctx(mctx)
                _, err := coll.InsertOne(mctx, bson.M{"ok": true, "ts": time.Now()})
                if err != nil { lg.WithError(err).Error("mongo insert"); c.JSON(500, gin.H{"error": err.Error()}); return }
                lg.Info("mongo inserted")
                c.JSON(200, gin.H{"ok": true})
            })
        } else {
            r.GET("/mongo", func(c *gin.Context) { c.JSON(501, gin.H{"error":"mongo not available"}) })
        }
    }

    // AMQP propagation (simulated on /amqp)
    r.GET("/amqp", func(c *gin.Context) {
        base := logtrc.GetLog(c)
        ctx, span := withSpanIf(base, c.Request.Context(), "publish")
        headers := map[string]interface{}{"x-demo":"1"} // stub headers
        span.Ctx(ctx).Fs(headers).Info("published")
        // consume side (simulated)
        span.Ctx(context.Background()).Fs(headers).Info("consumed")
        c.JSON(200, gin.H{"ok": true})
    })

    _ = http.ListenAndServe(*port, r)
}

func withSpanIf(l logtrc.LogBrt, ctx context.Context, name string) (context.Context, logtrc.LogBrt) {
    if ot, ok := l.(*logtrc.OTelLogBrt); ok {
        c, _ := ot.WithSpan(ctx, name)
        return c, ot
    }
    return ctx, l
}


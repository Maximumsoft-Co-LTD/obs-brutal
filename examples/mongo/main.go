package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "obs-brutal/internal/util"
    "obs-brutal/logtrc"

    "github.com/gin-gonic/gin"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	r := gin.New()
	r.Use(logtrc.Middleware("example-mongo"))

	// Real mongo client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		panic(err)
	}
	defer cli.Disconnect(context.Background())
	coll := cli.Database("demo").Collection("users")

    r.GET("/mongo", func(c *gin.Context) {
        ctx := util.WithTraceID(c.Request.Context(), "mongo-trace-001")
        log := logtrc.GetLog(c).Ctx(ctx)
		cur, err := coll.Find(ctx, bson.M{"active": true}, options.Find().SetLimit(10))
		if err != nil {
			log.WithError(err).Error("mongo find failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cur.Close(ctx)
		var out []bson.M
		if err := cur.All(ctx, &out); err != nil {
			log.WithError(err).Error("mongo decode failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
        log.F("environment", "dev").F("count", len(out)).Info("fetched")
		c.JSON(http.StatusOK, out)
	})

    r.POST("/mongo", func(c *gin.Context) {
        ctx := util.WithTraceID(c.Request.Context(), "mongo-trace-001")
        log := logtrc.GetLog(c).Ctx(ctx)
		doc := bson.M{"name": "john", "active": true, "ts": time.Now()}
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			log.WithError(err).Error("mongo insert failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
        log.F("environment", "dev").Info("inserted")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	srv := &http.Server{Addr: ":8083", Handler: r}
	go func() { _ = srv.ListenAndServe() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
}

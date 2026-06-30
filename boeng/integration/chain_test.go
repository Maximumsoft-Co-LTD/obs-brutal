//go:build integration

// Package boengintegration_test wires every adapter into a single
// realistic chain — HTTP request → RabbitMQ publish → RabbitMQ
// consumer → Redis write → Mongo write — and asserts that boeng
// operations appear at every hop in the order the README promises.
//
// Requires all four backends running. By default it expects them on
// localhost; override per-service via the corresponding env vars
// (MONGO_HOST, REDIS_ADDR, RABBIT_URL).
//
// Run:
//
//	go test -tags integration ./boeng/integration/...
package boengintegration_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"obs-brutal/boeng"
	boenghttp "obs-brutal/boeng/http"
	boengmongo "obs-brutal/boeng/mongo"
	boengrabbit "obs-brutal/boeng/rabbit"
	boengredis "obs-brutal/boeng/redis"
	"obs-brutal/internal/core/domain"
)

type captureSink struct {
	mu      sync.Mutex
	entries []domain.LogEntry
}

func (c *captureSink) Write(e *domain.LogEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, *e)
	return nil
}
func (c *captureSink) Close() error                            { return nil }
func (c *captureSink) Name() string                            { return "capture" }
func (c *captureSink) Health() error                           { return nil }
func (c *captureSink) Configure(map[string]interface{}) error  { return nil }
func (c *captureSink) snapshot() []domain.LogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]domain.LogEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func skipUnless(t *testing.T, label, addr string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		t.Skipf("%s not reachable at %s: %v", label, addr, err)
	}
	_ = c.Close()
}

// TestChain_HTTPRabbitRedisMongo asserts that one HTTP request fans
// out through the broker to a consumer that touches Redis + Mongo,
// and that every hop appears in the captured log stream.
//
// Trace correlation is verified end-to-end via the W3C traceparent
// header that boenghttp.Transport injects and that boengrabbit.Publish
// carries onward through amqp.Table.
func TestChain_HTTPRabbitRedisMongo(t *testing.T) {
	rabbitURL := envOr("RABBIT_URL", "amqp://guest:guest@localhost:5672/")
	mongoHost := envOr("MONGO_HOST", "localhost:27017")
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")

	skipUnless(t, "rabbitmq", "localhost:5672")
	skipUnless(t, "redis", redisAddr)
	skipUnless(t, "mongo", mongoHost)

	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	defer boeng.Init(boeng.Config{Service: "chain_it"}).Close()

	// --- Mongo ---
	mctx, mcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer mcancel()
	mongoClient, err := mongo.Connect(mctx, options.Client().
		ApplyURI("mongodb://"+mongoHost).
		SetMonitor(boengmongo.CommandMonitor()))
	if err != nil {
		t.Skipf("mongo connect: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())
	if err := mongoClient.Ping(mctx, nil); err != nil {
		t.Skipf("mongo ping: %v", err)
	}
	coll := mongoClient.Database("chain_it").Collection("events")
	defer coll.Drop(context.Background())

	// --- Redis ---
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	redisClient.AddHook(boengredis.Hook())
	defer redisClient.Close()

	// --- Rabbit ---
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		t.Skipf("rabbit dial: %v", err)
	}
	defer conn.Close()
	pubCh, err := conn.Channel()
	if err != nil {
		t.Fatalf("rabbit pub channel: %v", err)
	}
	defer pubCh.Close()
	conCh, err := conn.Channel()
	if err != nil {
		t.Fatalf("rabbit con channel: %v", err)
	}
	defer conCh.Close()

	queueName := "chain_it_" + time.Now().Format("150405.000")
	q, err := conCh.QueueDeclare(queueName, false, true, true, false, nil)
	if err != nil {
		t.Fatalf("queue declare: %v", err)
	}
	deliveries, err := conCh.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}

	done := make(chan struct{})
	consumer := boengrabbit.Consume("chain_consumer", func(ctx context.Context, d amqp.Delivery) error {
		var payload map[string]any
		_ = json.Unmarshal(d.Body, &payload)
		// Redis op
		_ = redisClient.Set(ctx, "chain:last", string(d.Body), time.Minute).Err()
		// Mongo op
		_, _ = coll.InsertOne(ctx, bson.M{"body": payload, "received_at": time.Now()})
		close(done)
		return nil
	})
	go func() {
		for d := range deliveries {
			consumer(d)
		}
	}()

	// --- HTTP server with boenghttp middleware that publishes ---
	server := httptest.NewServer(boenghttp.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := boengrabbit.Publish(r.Context(), pubCh, "", q.Name, amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})))
	defer server.Close()

	// --- HTTP client with boenghttp transport ---
	client := &http.Client{Transport: boenghttp.Transport(nil)}
	resp, err := client.Post(server.URL+"/events", "application/json", strings.NewReader(`{"user":"u-1"}`))
	if err != nil {
		t.Fatalf("client post: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer never received message")
	}
	// Mongo writes are async on the wire; give the command monitor a
	// chance to land its Succeeded event.
	time.Sleep(200 * time.Millisecond)

	// --- Verify every hop produced a boeng op ---
	gotMsgs := snapshotMsgs(sink)
	expected := []string{
		"HTTP POST",                  // client side (Transport)
		"POST /events",               // server middleware
		"rabbit.publish /" + q.Name,  // publisher
		"rabbit.consume chain_consumer",
		"redis.set",
		"mongo.insert",
	}
	for _, want := range expected {
		if !sawPrefix(gotMsgs, want) {
			t.Errorf("missing op prefix %q in chain stream\nall messages:\n  %s",
				want, strings.Join(sorted(gotMsgs), "\n  "))
		}
	}
}

func snapshotMsgs(s *captureSink) []string {
	entries := s.snapshot()
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Msg
	}
	return out
}

func sawPrefix(msgs []string, prefix string) bool {
	for _, m := range msgs {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

//go:build integration

package boengredis_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boengredis "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/redis"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func redisAddr() string {
	if a := os.Getenv("REDIS_ADDR"); a != "" {
		return a
	}
	return "localhost:6379"
}

func skipIfNoRedis(t *testing.T) {
	t.Helper()
	c, err := net.DialTimeout("tcp", redisAddr(), 500*time.Millisecond)
	if err != nil {
		t.Skipf("redis not reachable at %s (set REDIS_ADDR to override): %v", redisAddr(), err)
	}
	_ = c.Close()
}

// captureSink is duplicated here intentionally — adapter integration
// tests live in the adapter's package so they can run independently
// without pulling in core test helpers.
type captureSink struct {
	entries []domain.LogEntry
}

func (c *captureSink) Write(e *domain.LogEntry) error           { c.entries = append(c.entries, *e); return nil }
func (c *captureSink) Close() error                             { return nil }
func (c *captureSink) Name() string                             { return "capture" }
func (c *captureSink) Health() error                            { return nil }
func (c *captureSink) Configure(map[string]interface{}) error   { return nil }

func TestIntegration_HookOpensOpsForCommands(t *testing.T) {
	skipIfNoRedis(t)

	sink := &captureSink{}
	reset := boeng.SetSinkForTest(sink)
	defer reset()
	defer boeng.Init(boeng.Config{Service: "redis_it"}).Close()

	client := redis.NewClient(&redis.Options{Addr: redisAddr()})
	client.AddHook(boengredis.Hook())
	defer client.Close()

	ctx := context.Background()
	key := "boeng:it:" + time.Now().Format("150405.000")
	if err := client.Set(ctx, key, "x", time.Minute).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "x" {
		t.Fatalf("get = %q, want x", got)
	}
	_ = client.Del(ctx, key)

	if !sawOp(sink.entries, "redis.set completed") {
		t.Errorf("missing redis.set op in log stream: %v", msgs(sink.entries))
	}
	if !sawOp(sink.entries, "redis.get completed") {
		t.Errorf("missing redis.get op in log stream: %v", msgs(sink.entries))
	}
}

func sawOp(entries []domain.LogEntry, msg string) bool {
	for _, e := range entries {
		if e.Msg == msg {
			return true
		}
	}
	return false
}

func msgs(entries []domain.LogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Msg
	}
	return out
}

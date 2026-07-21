//go:build integration

package boengmongo_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boengmongo "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func mongoHost() string {
	if h := os.Getenv("MONGO_HOST"); h != "" {
		return h
	}
	return "localhost:27017"
}

func mongoURI() string {
	if u := os.Getenv("MONGO_URI"); u != "" {
		return u
	}
	return "mongodb://" + mongoHost()
}

// skipIfNoMongo skips the test when no actual MongoDB instance is
// responding to pings. A bare TCP probe isn't enough because something
// else on the system might hold port 27017.
func skipIfNoMongo(t *testing.T) {
	t.Helper()
	c, err := net.DialTimeout("tcp", mongoHost(), 500*time.Millisecond)
	if err != nil {
		t.Skipf("mongo not reachable at %s: %v", mongoHost(), err)
	}
	_ = c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI()))
	if err != nil {
		t.Skipf("mongo connect failed at %s: %v", mongoURI(), err)
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("mongo ping failed: %v", err)
	}
}

type captureSink struct {
	entries []domain.LogEntry
}

func (c *captureSink) Write(e *domain.LogEntry) error           { c.entries = append(c.entries, *e); return nil }
func (c *captureSink) Close() error                             { return nil }
func (c *captureSink) Name() string                             { return "capture" }
func (c *captureSink) Health() error                            { return nil }
func (c *captureSink) Configure(map[string]interface{}) error   { return nil }

func TestIntegration_CommandMonitorTracksCommands(t *testing.T) {
	skipIfNoMongo(t)

	sink := &captureSink{}
	reset := boeng.SetSinkForTest(sink)
	defer reset()
	defer boeng.Init(boeng.Config{Service: "mongo_it"}).Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI(mongoURI()).
		SetMonitor(boengmongo.CommandMonitor()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(context.Background())

	coll := client.Database("boeng_it").Collection("smoke")
	defer coll.Drop(context.Background())

	if _, err := coll.InsertOne(ctx, bson.M{"k": "v", "t": time.Now()}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := coll.FindOne(ctx, bson.M{"k": "v"}).Err(); err != nil {
		t.Fatalf("findOne: %v", err)
	}

	if !sawAnyMatching(sink.entries, "mongo.insert") {
		t.Errorf("no mongo.insert op in log stream: %v", msgs(sink.entries))
	}
	if !sawAnyMatching(sink.entries, "mongo.find") {
		t.Errorf("no mongo.find op in log stream: %v", msgs(sink.entries))
	}
}

func sawAnyMatching(entries []domain.LogEntry, prefix string) bool {
	for _, e := range entries {
		if len(e.Msg) >= len(prefix) && e.Msg[:len(prefix)] == prefix {
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

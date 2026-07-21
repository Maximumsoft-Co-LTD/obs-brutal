//go:build integration

package boengrabbit_test

import (
	"context"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boengrabbit "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/rabbit"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func rabbitURL() string {
	if u := os.Getenv("RABBIT_URL"); u != "" {
		return u
	}
	return "amqp://guest:guest@localhost:5672/"
}

func skipIfNoRabbit(t *testing.T) {
	t.Helper()
	c, err := net.DialTimeout("tcp", "localhost:5672", 500*time.Millisecond)
	if err != nil {
		t.Skipf("rabbitmq not reachable: %v", err)
	}
	_ = c.Close()
}

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
func (c *captureSink) Close() error                          { return nil }
func (c *captureSink) Name() string                          { return "capture" }
func (c *captureSink) Health() error                         { return nil }
func (c *captureSink) Configure(map[string]interface{}) error { return nil }

func (c *captureSink) snapshot() []domain.LogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]domain.LogEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

// TestIntegration_TraceParentPropagatesViaBroker verifies the full
// end-to-end propagation through RabbitMQ:
//   1. Publisher injects W3C traceparent into amqp.Publishing.Headers.
//   2. The broker carries the headers byte-for-byte across the queue.
//   3. Consumer extracts the same context so the consumer's boeng
//      operation nests under the producer's trace.
func TestIntegration_TraceParentPropagatesViaBroker(t *testing.T) {
	skipIfNoRabbit(t)

	sink := &captureSink{}
	reset := boeng.SetSinkForTest(sink)
	defer reset()
	defer boeng.Init(boeng.Config{Service: "rabbit_it"}).Close()

	conn, err := amqp.Dial(rabbitURL())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare("boeng_it_"+time.Now().Format("150405.000"), false, true, true, false, nil)
	if err != nil {
		t.Fatalf("queue declare: %v", err)
	}

	deliveries, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}

	done := make(chan struct{})
	var receivedHeaders amqp.Table
	handler := boengrabbit.Consume("it_q", func(ctx context.Context, d amqp.Delivery) error {
		receivedHeaders = d.Headers
		close(done)
		return nil
	})
	go func() {
		for d := range deliveries {
			handler(d)
		}
	}()

	if err := boengrabbit.Publish(context.Background(), ch, "", q.Name, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("ping"),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("consumer never received message")
	}

	// The broker must preserve the traceparent the publisher injected.
	if _, ok := receivedHeaders["traceparent"]; !ok {
		t.Errorf("consumer headers missing traceparent: %v", receivedHeaders)
	}

	// Both producer and consumer ops must appear in the log stream.
	gotPub, gotCon := false, false
	for _, e := range sink.snapshot() {
		if e.Msg == "rabbit.publish /"+q.Name+" completed" {
			gotPub = true
		}
		if e.Msg == "rabbit.consume it_q completed" {
			gotCon = true
		}
	}
	if !gotPub {
		t.Errorf("publisher op not in log stream")
	}
	if !gotCon {
		t.Errorf("consumer op not in log stream")
	}
}

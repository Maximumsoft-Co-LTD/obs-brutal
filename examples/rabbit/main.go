// RabbitMQ adapter demo. Requires a reachable broker at RABBIT_URL
// (defaults to amqp://guest:guest@localhost:5672/). Run:
//
//   go run ./examples/rabbit
//
// Demonstrates trace propagation across the broker: the publisher's
// operation and the consumer's operation share a trace_id because
// boengrabbit.Publish injects W3C traceparent into the message
// headers and boengrabbit.Consume extracts it on the other side.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boengrabbit "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/rabbit"
)

func main() {
	defer boeng.Init(boeng.Config{Service: "rabbit_demo", Env: "dev"}).Close()

	url := os.Getenv("RABBIT_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		fmt.Println("dial:", err)
		return
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("channel:", err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare("boeng_demo", false, true, false, false, nil)
	if err != nil {
		fmt.Println("declare:", err)
		return
	}

	// Consumer side
	deliveries, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		fmt.Println("consume:", err)
		return
	}
	handler := boengrabbit.Consume("boeng_demo", func(ctx context.Context, d amqp.Delivery) error {
		boeng.L(ctx).F("body", string(d.Body)).Info("processed")
		return nil
	})
	go func() {
		for d := range deliveries {
			handler(d)
		}
	}()

	// Publisher side
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		body := fmt.Sprintf("hello %d", i)
		_ = boengrabbit.Publish(ctx, ch, "", q.Name, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	}

	time.Sleep(500 * time.Millisecond)
}

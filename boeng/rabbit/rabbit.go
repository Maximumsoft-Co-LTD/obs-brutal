// Package boengrabbit instruments github.com/rabbitmq/amqp091-go with
// boeng's Operation Runtime. Publishers inject W3C trace context into
// AMQP message headers; consumers extract it so the consumer's
// operation becomes a child of the producer's trace.
//
// Business code never imports OpenTelemetry — Publish and Consume
// handle the propagation internally.
package boengrabbit

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// Publish wraps a basic publish with a boeng operation AND injects the
// W3C trace context into msg.Headers so consumers can pick the trace
// up. msg.Headers is created if nil.
//
//	err := boengrabbit.Publish(ctx, ch, "events", "user.created", amqp.Publishing{
//	    ContentType: "application/json",
//	    Body:        body,
//	})
func Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, msg amqp.Publishing) error {
	if msg.Headers == nil {
		msg.Headers = amqp.Table{}
	}
	return boeng.Run(ctx, "rabbit.publish "+exchange+"/"+key,
		publishFields{Exchange: exchange, RoutingKey: key},
		func(opCtx context.Context) error {
			otel.GetTextMapPropagator().Inject(opCtx, tableCarrier(msg.Headers))
			return ch.PublishWithContext(opCtx, exchange, key, false, false, msg)
		},
	)
}

// HandlerFunc is the callback signature consumers register with Consume.
// It receives a ctx carrying the message's restored trace context plus
// the original delivery.
type HandlerFunc func(ctx context.Context, d amqp.Delivery) error

// Consume wraps a delivery handler so each message becomes a boeng
// operation. The W3C trace context in d.Headers is extracted and
// attached to the ctx the handler receives.
//
//	deliveries, _ := ch.Consume(queue, "", true, false, false, false, nil)
//	handler := boengrabbit.Consume("user_events", func(ctx context.Context, d amqp.Delivery) error {
//	    return process(ctx, d.Body)
//	})
//	for d := range deliveries {
//	    handler(d)
//	}
func Consume(name string, fn HandlerFunc) func(amqp.Delivery) {
	return func(d amqp.Delivery) {
		ctx := otel.GetTextMapPropagator().Extract(
			context.Background(),
			tableCarrier(d.Headers),
		)
		_ = boeng.Run(ctx, "rabbit.consume "+name, consumeFields{
			Queue:      name,
			Exchange:   d.Exchange,
			RoutingKey: d.RoutingKey,
		}, func(opCtx context.Context) error {
			return fn(opCtx, d)
		})
	}
}

type publishFields struct {
	Exchange   string
	RoutingKey string
}

func (p publishFields) LogFields() map[string]any {
	return map[string]any{
		"messaging.system":             "rabbitmq",
		"messaging.destination":        p.Exchange,
		"messaging.rabbitmq.routing":   p.RoutingKey,
		"messaging.operation":          "publish",
	}
}

type consumeFields struct {
	Queue      string
	Exchange   string
	RoutingKey string
}

func (c consumeFields) LogFields() map[string]any {
	return map[string]any{
		"messaging.system":           "rabbitmq",
		"messaging.source":           c.Queue,
		"messaging.destination":      c.Exchange,
		"messaging.rabbitmq.routing": c.RoutingKey,
		"messaging.operation":        "process",
	}
}

// tableCarrier adapts amqp.Table to propagation.TextMapCarrier so the
// OTel propagator can read/write the W3C traceparent + tracestate
// entries directly on the message headers.
type tableCarrier amqp.Table

var _ propagation.TextMapCarrier = (tableCarrier)(nil)

func (c tableCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (c tableCarrier) Set(key, value string) {
	c[key] = value
}

func (c tableCarrier) Keys() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}

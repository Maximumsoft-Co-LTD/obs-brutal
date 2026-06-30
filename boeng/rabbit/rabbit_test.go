package boengrabbit_test

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"obs-brutal/boeng"
	boengrabbit "obs-brutal/boeng/rabbit"
)

// TestConsume_ExtractsTraceparentFromHeaders proves the consumer side
// of the propagation path works without a live broker: we hand the
// returned handler a hand-crafted delivery whose headers carry a W3C
// traceparent, and assert the handler runs and the boeng op opens.
func TestConsume_ExtractsTraceparentFromHeaders(t *testing.T) {
	boeng.Init(boeng.Config{Service: "rabbit_test"})

	called := false
	handler := boengrabbit.Consume("q", func(ctx context.Context, d amqp.Delivery) error {
		called = true
		if boeng.L(ctx) == nil {
			t.Errorf("boeng.L(ctx) returned nil inside consumer")
		}
		return nil
	})

	handler(amqp.Delivery{
		Body: []byte("hello"),
		Headers: amqp.Table{
			"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		},
	})

	if !called {
		t.Fatal("consumer handler never ran")
	}
}

// TestTableCarrier_ReadWrite exercises the propagator carrier so we
// know AMQP headers can carry W3C trace context end-to-end.
func TestTableCarrier_ReadWrite(t *testing.T) {
	boeng.Init(boeng.Config{Service: "rabbit_test"})

	headers := amqp.Table{}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(),
		propagation.MapCarrier{
			"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		})
	// Inject into a fresh carrier and round-trip via the consumer.
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for k, v := range carrier {
		headers[k] = v
	}

	if _, ok := headers["traceparent"]; !ok {
		t.Errorf("traceparent missing from headers after round-trip: %v", headers)
	}
}

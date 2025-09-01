package main

import (
	"context"
	"log"
	"time"

	"obs-brutal/logtrc"
)

// minimal AMQP Context Propagator for example build
type amqpProp struct{}

func (amqpProp) InjectAMQPHeaders(ctx context.Context, headers map[string]interface{}) {}
func (amqpProp) ExtractAMQPHeaders(ctx context.Context, headers map[string]interface{}) context.Context {
	return ctx
}

// This example shows how to propagate OTEL context via AMQP-like headers using AMQPContextPropagator.
func main() {
	// Create OTEL-enabled logbrut (adjust endpoint to your collector)
    otel, _, err := logtrc.NewOTelWithService("example-amqp", "1.0.0", "dev", "localhost:4317", logtrc.INFO)
    if err != nil {
        log.Fatalf("otel init: %v", err)
    }

	// Prepare a message
	ctx := context.Background()
    spanCtx, logWithSpan := otel.WithSpan(ctx, "publish")

	headers := map[string]interface{}{}
	propagator := amqpProp{}
	propagator.InjectAMQPHeaders(spanCtx, headers)

	// publish pseudo
	time.Sleep(5 * time.Millisecond)
    logWithSpan.Fs(headers).F("environment", "dev").Info("published message with trace headers")

	// consumer side: extract headers back
    consumeCtx := propagator.ExtractAMQPHeaders(context.Background(), headers)
    traceAware := otel.Ctx(consumeCtx)
    traceAware.F("environment", "dev").Info("consumed message and continued trace")
}

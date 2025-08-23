package main

import (
	"context"
	"log"
	"time"

	core "obs-brutal/internal/core"
	"obs-brutal/logtrc"
)

// This example shows how to propagate OTEL context via AMQP-like headers using AMQPContextPropagator.
func main() {
	// Create OTEL-enabled logger (adjust endpoint to your collector)
	otel, err := logtrc.NewOTelLogBrt("example-amqp", "1.0.0", "dev", "localhost:4317", logtrc.INFO)
	if err != nil {
		log.Fatalf("otel init: %v", err)
	}

	// Prepare a message
	ctx := context.Background()
	spanCtx, span, logWithSpan := otel.WithSpan(ctx, "publish")
	defer span.End()

	headers := map[string]interface{}{}
	propagator := core.NewAMQPContextPropagator()
	propagator.InjectAMQPHeaders(spanCtx, headers)

	// publish pseudo
	time.Sleep(5 * time.Millisecond)
	logWithSpan.Fs(headers).Info("published message with trace headers")

	// consumer side: extract headers back
	consumeCtx := propagator.ExtractAMQPHeaders(context.Background(), headers)
	traceAware := otel.Ctx(consumeCtx)
	traceAware.Info("consumed message and continued trace")
}

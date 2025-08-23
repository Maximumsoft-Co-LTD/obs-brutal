package main

import (
	"time"

	"obs-brutal/logtrc"
)

func main() {
	sinks := []logtrc.Sink{
		logtrc.NewConsoleSink(true),
		logtrc.NewOTLPSink("localhost:4317"),
		logtrc.NewLokiPushSink("http://localhost:3100/loki/api/v1/push", map[string]string{
			"app": "obs-brutal", "env": "dev",
		}),
		logtrc.NewZerologSink(),
	}

	log := logtrc.NewAsyncLogBrt(logtrc.INFO, sinks...)

	// Simple logs
	log.F("module", "example").Info("hello world")

	// Structured logs
	for i := 0; i < 5; i++ {
		log.F("iteration", i).
			F("success", true).
			F("tenant", "t1").
			Info("processing done")
		time.Sleep(100 * time.Millisecond)
	}
}

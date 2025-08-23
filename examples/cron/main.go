package main

import (
	"time"

	"obs-brutal/logtrc"
)

func main() {
	log := logtrc.New(logtrc.SrvName("example-cron"))

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 3; i++ { // simple demo
		<-ticker.C
		log.F("tick", i).Info("cron tick")
		// do work...
	}
}

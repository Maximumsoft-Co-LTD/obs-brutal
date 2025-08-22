package main

import (
	"obs-brutal/logtrc"
)

func main() {
	log := logtrc.NewDefault()
	log.Info("hello from examples/basic")
	log.F("user_id", 123).Info("structured log")
}

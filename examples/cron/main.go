package main

import (
    "time"

    "obs-brutal/logtrc"
)

func main() {
    var logger logtrc.LogBrt
    if ot, _, err := logtrc.NewOTelWithService("example-cron", "1.0.0", "dev", "localhost:4317", logtrc.INFO); err == nil && ot != nil {
        logger = ot
    } else {
        logger = logtrc.New(logtrc.SrvName("example-cron"))
    }

    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for i := 0; i < 3; i++ { // simple demo
        <-ticker.C
        logger.F("environment", "dev").F("tick", i).Info("cron tick")
        // do work...
    }
}

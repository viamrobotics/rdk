package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"time"

	"go.viam.com/robotcore/sensor/compass"

	"github.com/edaniels/golog"
)

func main() {
	var address string
	flag.StringVar(&address, "device", "", "device address")
	flag.Parse()

	if len(address) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	sensor, err := compass.NewWSDevice(context.Background(), address)
	if err != nil {
		golog.Global.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	tickRate := 100 * time.Millisecond
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	count := 0
	start := time.Now()
READ:
	for {
		select {
		case <-sig:
			break READ
		default:
		}
		select {
		case <-sig:
			break READ
		case <-ticker.C:
		}
		heading, err := sensor.Heading(context.Background())
		if err != nil {
			golog.Global.Errorw("failed to get sensor heading", "error", err)
			continue
		}
		golog.Global.Infow("heading", "data", heading)
	}
	golog.Global.Infow("stats", "rate", float64(count)/time.Since(start).Seconds())
}

package main

import (
	"flag"
	"os"
	"os/signal"
	"time"

	"github.com/viamrobotics/robotcore/sensor/compass/mti"

	"github.com/edaniels/golog"
)

func main() {
	var disableRateLimit bool
	flag.BoolVar(&disableRateLimit, "disable-rate-limit", false, "disable rate limiting")
	var disablePrint bool
	flag.BoolVar(&disablePrint, "disable-print", false, "disable printing data")
	flag.Parse()

	sensor, err := mti.New("02782090", "/dev/ttyUSB0", 115200)
	if err != nil {
		golog.Global.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	var tickRate time.Duration
	if disableRateLimit {
		tickRate = time.Nanosecond
	} else {
		tickRate = 100 * time.Millisecond
	}
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
		heading, err := sensor.Heading()
		if err != nil {
			golog.Global.Errorw("failed to get sensor heading", "error", err)
			continue
		}
		count++
		if !disablePrint {
			golog.Global.Infow("heading", "data", heading)
		}
	}
	golog.Global.Infow("stats", "rate", float64(count)/time.Since(start).Seconds())
}

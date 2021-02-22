package main

import (
	"time"

	"github.com/viamrobotics/robotcore/sensor/compass/mti"

	"github.com/edaniels/golog"
)

func main() {
	sensor, err := mti.New("02782090", "/dev/ttyUSB0", 115200)
	if err != nil {
		golog.Global.Fatal(err)
	}

	for {
		time.Sleep(100 * time.Millisecond)
		readings, err := sensor.Readings()
		if err != nil {
			golog.Global.Errorw("failed to get sensor reading", "error", err)
			continue
		}
		golog.Global.Infow("readings", "data", readings)
		heading, err := sensor.Heading()
		if err != nil {
			golog.Global.Errorw("failed to get sensor heading", "error", err)
			continue
		}
		golog.Global.Infow("heading", "data", heading)
	}
}

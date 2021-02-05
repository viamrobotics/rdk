package main

import (
	"time"

	"github.com/viamrobotics/robotcore/sensor/compass/gy511"
	"github.com/viamrobotics/robotcore/serial"

	"github.com/edaniels/golog"
)

func main() {
	devices, err := serial.SearchDevices(serial.SearchFilter{Type: serial.DeviceTypeArduino})
	if err != nil {
		golog.Global.Fatal(err)
	}
	if len(devices) == 0 {
		golog.Global.Fatal("no applicable device found")
	}
	sensor, err := gy511.New(devices[0].Path)
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

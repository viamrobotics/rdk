package main

import (
	"github.com/viamrobotics/robotcore/serial"

	"github.com/edaniels/golog"
)

func main() {
	devices, err := serial.SearchDevices(serial.SearchFilter{})
	if err != nil {
		golog.Global.Fatal(err)
	}
	golog.Global.Info(devices)
	// compassSensors, err := sensors.SearchDevices(sensors.SearchFilter{Type: SensorTypeCompass})
	// if err != nil {
	// 	golog.Global.Fatal(err)
	// }

	// for {
	// 	time.Sleep(100 * time.Millisecond)
	// 	for i, sensor := range compassSensors {
	// 		readings, err := sensor.Readings()
	// 		if err != nil {
	// 			golog.Global.Errorw("failed to get sensor reading", "num", i, "error", err)
	// 			continue
	// 		}
	// 		golog.Global.Infow("readings", "num", i, "data", readings)
	// 	}
	// }
}

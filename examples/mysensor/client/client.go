// Package main tests out the mySensor component type.
package main

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := golog.NewDebugLogger("client")
	robot, err := client.New(
		context.Background(),
		"localhost:8080",
		logger,
	)
	if err != nil {
		logger.Fatal(err)
	}

	// we can get the custom sensor here by name and use it like any other sensor.
	sensor, err := sensor.FromRobot(robot, "sensor1")
	if err != nil {
		logger.Error(err)
	}
	reading, err := sensor.Readings(context.Background())
	if err != nil {
		logger.Error(err)
	}
	logger.Info(reading)
}

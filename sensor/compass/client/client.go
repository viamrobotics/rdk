package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/client"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
)

const ModelNameClient = "grpc"

func init() {
	api.RegisterSensor(compass.DeviceType, ModelNameClient, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (sensor.Device, error) {
		address := config.Host
		if config.Port != 0 {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		return New(ctx, address, logger)
	})
}

func New(ctx context.Context, address string, logger golog.Logger) (compass.Device, error) {
	robotClient, err := apiclient.NewRobotClient(ctx, address, logger)
	if err != nil {
		return nil, err
	}
	names := robotClient.SensorNames()
	if len(names) == 0 {
		return nil, errors.New("no sensor devices found")
	}
	var compassDevice compass.Device
	for _, name := range names {
		sensorDevice := robotClient.SensorByName(name)
		if c, ok := sensorDevice.(compass.Device); ok {
			compassDevice = c
			break
		}
	}
	if compassDevice == nil {
		return nil, errors.New("no compass devices found")
	}

	if rel, ok := compassDevice.(compass.RelativeDevice); ok {
		return &wrappedRelativeCompassDevice{rel, robotClient}, nil
	}
	return &wrappedCompassDevice{compassDevice, robotClient}, nil
}

type wrappedCompassDevice struct {
	compass.Device
	robotClient *client.RobotClient
}

func (wcd *wrappedCompassDevice) Close() error {
	return wcd.robotClient.Close()
}

type wrappedRelativeCompassDevice struct {
	compass.RelativeDevice
	robotClient *client.RobotClient
}

func (wrcd *wrappedRelativeCompassDevice) Close() error {
	return wrcd.robotClient.Close()
}

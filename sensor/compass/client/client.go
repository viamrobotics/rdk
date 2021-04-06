package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/robotcore/api"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
)

const ModelNameClient = "grpc"

func init() {
	api.RegisterSensor(compass.DeviceType, ModelNameClient, func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (sensor.Device, error) {
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
	robotClient api.Robot
}

func (wcd *wrappedCompassDevice) Close(ctx context.Context) (err error) {
	defer func() {
		err = multierr.Combine(err, wcd.robotClient.Close(ctx))
	}()
	return wcd.Device.Close(ctx)
}

type wrappedRelativeCompassDevice struct {
	compass.RelativeDevice
	robotClient api.Robot
}

func (wrcd *wrappedRelativeCompassDevice) Close(ctx context.Context) (err error) {
	defer func() {
		err = multierr.Combine(err, wrcd.robotClient.Close(ctx))
	}()
	return wrcd.RelativeDevice.Close(ctx)
}

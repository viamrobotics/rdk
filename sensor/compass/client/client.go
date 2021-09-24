// Package client contains a gRPC based sensor.Compass client.
package client

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/client"
	grpcclient "go.viam.com/core/grpc/client"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// ModelNameClient is used to register the sensor to a model name.
const ModelNameClient = "grpc"

// init registers a gRPC based compass.
func init() {
	registry.RegisterSensor(compass.Type, ModelNameClient, registry.Sensor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		address := config.Host
		if config.Port != 0 {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		return New(ctx, address, logger)
	}})
}

// New returns a gRPC based compass at the given address. It properly returns an underlying
// traditional or relative compass.
func New(ctx context.Context, address string, logger golog.Logger) (compass.Compass, error) {
	robotClient, err := grpcclient.NewClient(ctx, address, logger)
	if err != nil {
		return nil, err
	}
	names := robotClient.SensorNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no sensor devices found"), robotClient.Close())
	}
	var compassDevice compass.Compass
	for _, name := range names {
		sensorDevice, ok := robotClient.SensorByName(name)
		if !ok {
			continue
		}
		if c, ok := sensorDevice.(compass.Compass); ok {
			compassDevice = c
			break
		}
	}
	if compassDevice == nil {
		return nil, multierr.Combine(errors.New("no compass devices found"), robotClient.Close())
	}

	if rel, ok := compassDevice.(compass.RelativeCompass); ok {
		return &wrappedRelativeCompassDevice{rel, robotClient}, nil
	}
	return &wrappedCompassDevice{compassDevice, robotClient}, nil
}

type wrappedCompassDevice struct {
	compass.Compass
	robotClient *client.RobotClient
}

func (wcd *wrappedCompassDevice) Close() error {
	return wcd.robotClient.Close()
}

type wrappedRelativeCompassDevice struct {
	compass.RelativeCompass
	robotClient *client.RobotClient
}

func (wrcd *wrappedRelativeCompassDevice) Close() error {
	return wrcd.robotClient.Close()
}

package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/client"
	grpcclient "go.viam.com/core/grpc/client"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/imu"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// ModelNameClient is used to register the sensor to a model name.
const ModelNameClient = "imu"

// init registers an imu.
func init() {
	registry.RegisterSensor(imu.Type, ModelNameClient, registry.Sensor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		address := config.Host
		if config.Port != 0 {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		return New(ctx, address, logger)
	}})
}

// New returns an IMU sensor at the given address..
func New(ctx context.Context, address string, logger golog.Logger) (imu.IMU, error) {
	// still using gRPC client? seems wrong
	robotClient, err := grpcclient.NewClient(ctx, address, logger)
	if err != nil {
		return nil, err
	}
	names := robotClient.SensorNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no sensor devices found"), robotClient.Close())
	}
	var imuDevice imu.IMU
	namesStr := strings.Join(names, ", ")
	for _, name := range names {
		sensorDevice, ok := robotClient.SensorByName(name)
		if !ok {
			continue
		}
		if c, ok := sensorDevice.(imu.IMU); ok {
			imuDevice = c
			break
		}
	}
	if imuDevice == nil {
		// return nil, multierr.Combine(errors.New("no imu devices found"), robotClient.Close())
		return nil, multierr.Combine(errors.New("no imu devices found, devices: "+namesStr), robotClient.Close())
	}
	// TODO: handle multiple imu device types?

	return &wrappedImuDevice{imuDevice, robotClient}, nil
}

type wrappedImuDevice struct {
	imu.IMU
	robotClient *client.RobotClient
}

func (wcd *wrappedImuDevice) Close() error {
	return wcd.robotClient.Close()
}

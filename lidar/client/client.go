package client

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/client"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/lidar"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const ModelNameClient = "grpc"
const DeviceTypeClient = lidar.DeviceType(ModelNameClient)

func init() {
	api.RegisterLidarDevice(ModelNameClient, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (lidar.Device, error) {
		address := config.Host
		if config.Port != 0 {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		return NewClient(ctx, address, logger)
	})
}

func NewClient(ctx context.Context, address string, logger golog.Logger) (lidar.Device, error) {
	robotClient, err := apiclient.NewRobotClient(ctx, address, logger)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to lidar server (%s): %w", address, err)
	}
	names := robotClient.LidarDeviceNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no lidar devices found"), robotClient.Close())
	}
	lidarDevice := robotClient.LidarDeviceByName(names[0])
	return &wrappedLidarDevice{lidarDevice, robotClient}, nil
}

type wrappedLidarDevice struct {
	lidar.Device
	robotClient *client.RobotClient
}

func (wld *wrappedLidarDevice) Close() error {
	return wld.robotClient.Close()
}

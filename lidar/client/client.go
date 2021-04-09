package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/client"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/lidar"
)

const ModelNameClient = "grpc"
const DeviceTypeClient = lidar.DeviceType(ModelNameClient)

func init() {
	api.RegisterLidarDevice(ModelNameClient, func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (lidar.Device, error) {
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
		return nil, err
	}
	names := robotClient.LidarDeviceNames()
	if len(names) == 0 {
		return nil, errors.New("no lidar devices found")
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

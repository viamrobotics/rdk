package client

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/robotcore/api"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/lidar"
)

const ModelNameClient = "grpc"
const DeviceTypeClient = lidar.DeviceType("grpc")

func init() {
	lidar.RegisterDeviceType(DeviceTypeClient, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			robotClient, err := apiclient.NewRobotClient(ctx, desc.Path, logger)
			if err != nil {
				return nil, err
			}
			names := robotClient.LidarDeviceNames()
			if len(names) == 0 {
				return nil, errors.New("no lidar devices found")
			}
			lidarDevice := robotClient.LidarDeviceByName(names[0])
			return &wrappedLidarDevice{lidarDevice, robotClient}, nil
		},
	})
}

type wrappedLidarDevice struct {
	lidar.Device
	robotClient api.Robot
}

func (wld *wrappedLidarDevice) Close(ctx context.Context) (err error) {
	defer func() {
		err = multierr.Combine(err, wld.robotClient.Close(ctx))
	}()
	return wld.Device.Close(ctx)
}

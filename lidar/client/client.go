// Package client contains a gRPC based lidar.Device client.
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

// ModelNameClient is used to refer to the registered type of lidar.
const ModelNameClient = "grpc"

// init registers the gRPC lidar client.
func init() {
	api.RegisterLidar(ModelNameClient, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (lidar.Device, error) {
		address := config.Host
		if config.Port != 0 {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		return NewClient(ctx, address, logger)
	})
}

// NewClient returns a lidar backed by a gRPC client.
func NewClient(ctx context.Context, address string, logger golog.Logger) (lidar.Device, error) {
	robotClient, err := apiclient.NewRobotClient(ctx, address, logger)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to lidar server (%s): %w", address, err)
	}
	names := robotClient.LidarNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no lidars found"), robotClient.Close())
	}
	lidar := robotClient.LidarByName(names[0])
	return &wrappedLidar{lidar, robotClient}, nil
}

type wrappedLidar struct {
	lidar.Device
	robotClient *client.RobotClient
}

func (wld *wrappedLidar) Close() error {
	return wld.robotClient.Close()
}

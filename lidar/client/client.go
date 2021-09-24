// Package client contains a gRPC based lidar.Lidar client.
package client

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/client"
	grpcclient "go.viam.com/core/grpc/client"
	"go.viam.com/core/lidar"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// ModelNameClient is used to refer to the registered type of lidar.
const ModelNameClient = "grpc"

// init registers the gRPC lidar client.
func init() {
	registry.RegisterLidar(ModelNameClient, registry.Lidar{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		address := config.Host
		if config.Port != 0 {
			address = fmt.Sprintf("%s:%d", address, config.Port)
		}
		return NewClient(ctx, address, logger)
	}})
}

// NewClient returns a lidar backed by a gRPC client.
func NewClient(ctx context.Context, address string, logger golog.Logger) (lidar.Lidar, error) {
	robotClient, err := grpcclient.NewClient(ctx, address, logger)
	if err != nil {
		return nil, errors.Errorf("couldn't connect to lidar server (%s): %w", address, err)
	}
	names := robotClient.LidarNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no lidars found"), robotClient.Close())
	}
	lidar, ok := robotClient.LidarByName(names[0])
	if !ok {
		return nil, fmt.Errorf("failed to find lidar %q", names[0])
	}
	return &wrappedLidar{lidar, robotClient}, nil
}

type wrappedLidar struct {
	lidar.Lidar
	robotClient *client.RobotClient
}

func (wld *wrappedLidar) Close() error {
	return wld.robotClient.Close()
}

// Package client contains a gRPC based sensor.Sensor client.
package client

import (
	"context"
	"errors"

	"go.viam.com/core/grpc/client"
	grpcclient "go.viam.com/core/grpc/client"
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func NewClient(ctx context.Context, address string, logger golog.Logger) (*SensorClient, error) {
	robotClient, err := grpcclient.NewClient(ctx, address, logger)
	if err != nil {
		return nil, err
	}
	names := robotClient.SensorNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no sensor devices found"), robotClient.Close())
	}
	return &SensorClient{robotClient.SensorByName(names[0]), robotClient}, nil
}

// A SensorClient represents a sensor that is controlled via gRPC.
type SensorClient struct {
	sensor.Sensor
	robotClient *client.RobotClient
}

// Wrapped returns the underlying sensor device if more type specific
// access is required.
func (sc *SensorClient) Wrapped() sensor.Sensor {
	return sc.Sensor
}

// Close cleanly closes the underlying connection.
func (sc *SensorClient) Close() error {
	return sc.robotClient.Close()
}

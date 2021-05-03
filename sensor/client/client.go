package client

import (
	"context"
	"errors"

	"go.viam.com/robotcore/api/client"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/sensor"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func NewClient(ctx context.Context, address string, logger golog.Logger) (*SensorClient, error) {
	robotClient, err := apiclient.NewRobotClient(ctx, address, logger)
	if err != nil {
		return nil, err
	}
	names := robotClient.SensorNames()
	if len(names) == 0 {
		return nil, multierr.Combine(errors.New("no sensor devices found"), robotClient.Close())
	}
	return &SensorClient{robotClient.SensorByName(names[0]), robotClient}, nil
}

type SensorClient struct {
	sensor.Device
	robotClient *client.RobotClient
}

func (sc *SensorClient) Wrapped() sensor.Device {
	return sc.Device
}

func (sc *SensorClient) Close() error {
	return sc.robotClient.Close()
}

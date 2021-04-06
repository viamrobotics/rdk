package client

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/robotcore/api"
	apiclient "go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/sensor"
)

func NewClient(ctx context.Context, address string, logger golog.Logger) (*SensorClient, error) {
	robotClient, err := apiclient.NewRobotClient(ctx, address, logger)
	if err != nil {
		return nil, err
	}
	names := robotClient.SensorNames()
	if len(names) == 0 {
		return nil, errors.New("no sensor devices found")
	}
	return &SensorClient{robotClient.SensorByName(names[0]), robotClient}, nil
}

type SensorClient struct {
	sensor.Device
	robotClient api.Robot
}

func (sc *SensorClient) Wrapped() sensor.Device {
	return sc.Device
}

func (sc *SensorClient) Close(ctx context.Context) (err error) {
	defer func() {
		err = multierr.Combine(err, sc.robotClient.Close(ctx))
	}()
	return sc.Device.Close(ctx)
}

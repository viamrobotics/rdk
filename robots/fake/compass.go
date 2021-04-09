package fake

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
)

func init() {
	api.RegisterSensor(compass.DeviceType, "fake", func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (sensor.Device, error) {
		if config.Attributes.GetBool("relative", false) {
			return &RelativeCompass{&Compass{}}, nil
		}
		return &Compass{}, nil
	})
}

type Compass struct {
}

func (c *Compass) Readings(ctx context.Context) ([]interface{}, error) {
	return []interface{}{1.2}, nil
}

func (c *Compass) Close() error {
	return nil
}

func (c *Compass) Heading(ctx context.Context) (float64, error) {
	return 1.2, nil
}

func (c *Compass) StartCalibration(ctx context.Context) error {
	return nil
}

func (c *Compass) StopCalibration(ctx context.Context) error {
	return nil
}

type RelativeCompass struct {
	*Compass
}

func (rc *RelativeCompass) Mark(ctx context.Context) error {
	return nil
}

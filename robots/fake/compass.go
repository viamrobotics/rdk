package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
)

func init() {
	registry.RegisterSensor(compass.CompassType, "fake", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		if config.Attributes.Bool("relative", false) {
			return &RelativeCompass{&Compass{Name: config.Name}}, nil
		}
		return &Compass{Name: config.Name}, nil
	})
}

type Compass struct {
	Name string
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

func (c *Compass) Desc() sensor.Description {
	return sensor.Description{compass.CompassType, ""}
}

type RelativeCompass struct {
	*Compass
}

func (rc *RelativeCompass) Mark(ctx context.Context) error {
	return nil
}

func (rc *RelativeCompass) Desc() sensor.Description {
	return sensor.Description{compass.RelativeCompassType, ""}
}

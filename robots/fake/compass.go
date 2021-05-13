package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/registry"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
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

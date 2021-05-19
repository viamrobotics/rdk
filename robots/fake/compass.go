package fake

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
)

func init() {
	registry.RegisterSensor(compass.Type, "fake", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		if config.Attributes.Bool("relative", false) {
			return &RelativeCompass{&Compass{Name: config.Name}}, nil
		}
		return &Compass{Name: config.Name}, nil
	})
}

// Compass is a fake compass that always returns the same readings.
type Compass struct {
	Name string
}

// Readings always returns the same values.
func (c *Compass) Readings(ctx context.Context) ([]interface{}, error) {
	return []interface{}{1.2}, nil
}

// Close does nothing.
func (c *Compass) Close() error {
	return nil
}

// Heading always returns the same value.
func (c *Compass) Heading(ctx context.Context) (float64, error) {
	return 1.2, nil
}

// StartCalibration does nothing.
func (c *Compass) StartCalibration(ctx context.Context) error {
	return nil
}

// StopCalibration does nothing.
func (c *Compass) StopCalibration(ctx context.Context) error {
	return nil
}

// Desc returns that this is a traditional compass.
func (c *Compass) Desc() sensor.Description {
	return sensor.Description{compass.Type, ""}
}

// Reconfigure replaces this compass with the given compass.
func (c *Compass) Reconfigure(newCompass sensor.Sensor) {
	actual, ok := newCompass.(*Compass)
	if !ok {
		panic(fmt.Errorf("expected new compass to be %T but got %T", actual, newCompass))
	}
	if err := c.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	*c = *actual
}

// RelativeCompass is a fake relative compass that always returns the same readings.
type RelativeCompass struct {
	*Compass
}

// Mark does nothing.
func (rc *RelativeCompass) Mark(ctx context.Context) error {
	return nil
}

// Desc returns that this is a relative compass.
func (rc *RelativeCompass) Desc() sensor.Description {
	return sensor.Description{compass.RelativeType, ""}
}

// Reconfigure replaces this compass with the given compass.
func (rc *RelativeCompass) Reconfigure(newCompass sensor.Sensor) {
	actual, ok := newCompass.(*RelativeCompass)
	if !ok {
		panic(fmt.Errorf("expected new compass to be %T but got %T", actual, newCompass))
	}
	if err := rc.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	*rc = *actual
}

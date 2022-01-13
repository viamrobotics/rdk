package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/compass"
	"go.viam.com/rdk/sensor"
)

// Compass is an injected compass.
type Compass struct {
	compass.Compass
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	DescFunc             func() sensor.Description
	CloseFunc            func(ctx context.Context) error
}

// Heading calls the injected Heading or the real version.
func (c *Compass) Heading(ctx context.Context) (float64, error) {
	if c.HeadingFunc == nil {
		return c.Compass.Heading(ctx)
	}
	return c.HeadingFunc(ctx)
}

// StartCalibration calls the injected StartCalibration or the real version.
func (c *Compass) StartCalibration(ctx context.Context) error {
	if c.StartCalibrationFunc == nil {
		return c.Compass.StartCalibration(ctx)
	}
	return c.StartCalibrationFunc(ctx)
}

// StopCalibration calls the injected StopCalibration or the real version.
func (c *Compass) StopCalibration(ctx context.Context) error {
	if c.StopCalibrationFunc == nil {
		return c.Compass.StopCalibration(ctx)
	}
	return c.StopCalibrationFunc(ctx)
}

// Readings calls the injected Readings or the real version.
func (c *Compass) Readings(ctx context.Context) ([]interface{}, error) {
	if c.ReadingsFunc == nil {
		return c.Compass.Readings(ctx)
	}
	return c.ReadingsFunc(ctx)
}

// Desc calls the injected Desc or the real version.
func (c *Compass) Desc() sensor.Description {
	return sensor.Description{sensor.Type(compass.SubtypeName), ""}
}

// Close calls the injected Close or the real version.
func (c *Compass) Close(ctx context.Context) error {
	if c.CloseFunc == nil {
		return utils.TryClose(ctx, c.Compass)
	}
	return c.CloseFunc(ctx)
}

// RelativeCompass is an injected relative compass.
type RelativeCompass struct {
	Compass
	MarkFunc func(ctx context.Context) error
}

// Mark calls the injected Mark or the real version.
func (rc *RelativeCompass) Mark(ctx context.Context) error {
	if rc.MarkFunc == nil {
		return rc.Mark(ctx)
	}
	return rc.MarkFunc(ctx)
}

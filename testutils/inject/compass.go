package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
)

// Compass is an injected compass.
type Compass struct {
	compass.Compass
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	CloseFunc            func() error
}

// Readings calls the injected Readings or the real version.
func (c *Compass) Readings(ctx context.Context) ([]interface{}, error) {
	if c.ReadingsFunc == nil {
		return c.Compass.Readings(ctx)
	}
	return c.ReadingsFunc(ctx)
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

// Desc calls the injected Desc or the real version.
func (c *Compass) Desc() sensor.Description {
	return sensor.Description{compass.Type, ""}
}

// Close calls the injected Close or the real version.
func (c *Compass) Close() error {
	if c.CloseFunc == nil {
		return utils.TryClose(c.Compass)
	}
	return c.CloseFunc()
}

// RelativeCompass is an injected relative compass.
type RelativeCompass struct {
	compass.RelativeCompass
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	MarkFunc             func(ctx context.Context) error
	CloseFunc            func() error
}

// Readings calls the injected Readings or the real version.
func (rc *RelativeCompass) Readings(ctx context.Context) ([]interface{}, error) {
	if rc.ReadingsFunc == nil {
		return rc.RelativeCompass.Readings(ctx)
	}
	return rc.ReadingsFunc(ctx)
}

// Heading calls the injected Heading or the real version.
func (rc *RelativeCompass) Heading(ctx context.Context) (float64, error) {
	if rc.HeadingFunc == nil {
		return rc.RelativeCompass.Heading(ctx)
	}
	return rc.HeadingFunc(ctx)
}

// StartCalibration calls the injected StartCalibration or the real version.
func (rc *RelativeCompass) StartCalibration(ctx context.Context) error {
	if rc.StartCalibrationFunc == nil {
		return rc.RelativeCompass.StartCalibration(ctx)
	}
	return rc.StartCalibrationFunc(ctx)
}

// StopCalibration calls the injected StopCalibration or the real version.
func (rc *RelativeCompass) StopCalibration(ctx context.Context) error {
	if rc.StopCalibrationFunc == nil {
		return rc.RelativeCompass.StopCalibration(ctx)
	}
	return rc.StopCalibrationFunc(ctx)
}

// Mark calls the injected Mark or the real version.
func (rc *RelativeCompass) Mark(ctx context.Context) error {
	if rc.MarkFunc == nil {
		return rc.RelativeCompass.Mark(ctx)
	}
	return rc.MarkFunc(ctx)
}

// Close calls the injected Close or the real version.
func (rc *RelativeCompass) Close() error {
	if rc.CloseFunc == nil {
		return utils.TryClose(rc.RelativeCompass)
	}
	return rc.CloseFunc()
}

// Desc calls the injected Desc or the real version.
func (rc *RelativeCompass) Desc() sensor.Description {
	return sensor.Description{compass.RelativeType, ""}
}

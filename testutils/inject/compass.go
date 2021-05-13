package inject

import (
	"context"

	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/utils"
)

type Compass struct {
	compass.Compass
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	CloseFunc            func() error
}

func (c *Compass) Readings(ctx context.Context) ([]interface{}, error) {
	if c.ReadingsFunc == nil {
		return c.Compass.Readings(ctx)
	}
	return c.ReadingsFunc(ctx)
}

func (c *Compass) Heading(ctx context.Context) (float64, error) {
	if c.HeadingFunc == nil {
		return c.Compass.Heading(ctx)
	}
	return c.HeadingFunc(ctx)
}

func (c *Compass) StartCalibration(ctx context.Context) error {
	if c.StartCalibrationFunc == nil {
		return c.Compass.StartCalibration(ctx)
	}
	return c.StartCalibrationFunc(ctx)
}

func (c *Compass) StopCalibration(ctx context.Context) error {
	if c.StopCalibrationFunc == nil {
		return c.Compass.StopCalibration(ctx)
	}
	return c.StopCalibrationFunc(ctx)
}

func (c *Compass) Desc() sensor.Description {
	return sensor.Description{compass.CompassType, ""}
}

func (c *Compass) Close() error {
	if c.CloseFunc == nil {
		return utils.TryClose(c.Compass)
	}
	return c.CloseFunc()
}

type RelativeCompass struct {
	compass.RelativeCompass
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	MarkFunc             func(ctx context.Context) error
	CloseFunc            func() error
}

func (rc *RelativeCompass) Readings(ctx context.Context) ([]interface{}, error) {
	if rc.ReadingsFunc == nil {
		return rc.RelativeCompass.Readings(ctx)
	}
	return rc.ReadingsFunc(ctx)
}

func (rc *RelativeCompass) Heading(ctx context.Context) (float64, error) {
	if rc.HeadingFunc == nil {
		return rc.RelativeCompass.Heading(ctx)
	}
	return rc.HeadingFunc(ctx)
}

func (rc *RelativeCompass) StartCalibration(ctx context.Context) error {
	if rc.StartCalibrationFunc == nil {
		return rc.RelativeCompass.StartCalibration(ctx)
	}
	return rc.StartCalibrationFunc(ctx)
}

func (rc *RelativeCompass) StopCalibration(ctx context.Context) error {
	if rc.StopCalibrationFunc == nil {
		return rc.RelativeCompass.StopCalibration(ctx)
	}
	return rc.StopCalibrationFunc(ctx)
}

func (rc *RelativeCompass) Mark(ctx context.Context) error {
	if rc.MarkFunc == nil {
		return rc.RelativeCompass.Mark(ctx)
	}
	return rc.MarkFunc(ctx)
}

func (rc *RelativeCompass) Close() error {
	if rc.CloseFunc == nil {
		return utils.TryClose(rc.RelativeCompass)
	}
	return rc.CloseFunc()
}

func (rc *RelativeCompass) Desc() sensor.Description {
	return sensor.Description{compass.RelativeCompassType, ""}
}

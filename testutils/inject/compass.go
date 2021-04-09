package inject

import (
	"context"

	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"
)

type Compass struct {
	compass.Device
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	CloseFunc            func() error
}

func (c *Compass) Readings(ctx context.Context) ([]interface{}, error) {
	if c.ReadingsFunc == nil {
		return c.Device.Readings(ctx)
	}
	return c.ReadingsFunc(ctx)
}

func (c *Compass) Heading(ctx context.Context) (float64, error) {
	if c.HeadingFunc == nil {
		return c.Device.Heading(ctx)
	}
	return c.HeadingFunc(ctx)
}

func (c *Compass) StartCalibration(ctx context.Context) error {
	if c.StartCalibrationFunc == nil {
		return c.Device.StartCalibration(ctx)
	}
	return c.StartCalibrationFunc(ctx)
}

func (c *Compass) StopCalibration(ctx context.Context) error {
	if c.StopCalibrationFunc == nil {
		return c.Device.StopCalibration(ctx)
	}
	return c.StopCalibrationFunc(ctx)
}

func (c *Compass) Close() error {
	if c.CloseFunc == nil {
		return utils.TryClose(c.Device)
	}
	return c.CloseFunc()
}

type RelativeCompass struct {
	compass.RelativeDevice
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	MarkFunc             func(ctx context.Context) error
	CloseFunc            func() error
}

func (rc *RelativeCompass) Readings(ctx context.Context) ([]interface{}, error) {
	if rc.ReadingsFunc == nil {
		return rc.RelativeDevice.Readings(ctx)
	}
	return rc.ReadingsFunc(ctx)
}

func (rc *RelativeCompass) Heading(ctx context.Context) (float64, error) {
	if rc.HeadingFunc == nil {
		return rc.RelativeDevice.Heading(ctx)
	}
	return rc.HeadingFunc(ctx)
}

func (rc *RelativeCompass) StartCalibration(ctx context.Context) error {
	if rc.StartCalibrationFunc == nil {
		return rc.RelativeDevice.StartCalibration(ctx)
	}
	return rc.StartCalibrationFunc(ctx)
}

func (rc *RelativeCompass) StopCalibration(ctx context.Context) error {
	if rc.StopCalibrationFunc == nil {
		return rc.RelativeDevice.StopCalibration(ctx)
	}
	return rc.StopCalibrationFunc(ctx)
}

func (rc *RelativeCompass) Mark(ctx context.Context) error {
	if rc.MarkFunc == nil {
		return rc.RelativeDevice.Mark(ctx)
	}
	return rc.MarkFunc(ctx)
}

func (rc *RelativeCompass) Close() error {
	if rc.CloseFunc == nil {
		return utils.TryClose(rc.RelativeDevice)
	}
	return rc.CloseFunc()
}

package inject

import (
	"context"
	"image"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/utils"
)

type LidarDevice struct {
	lidar.Device
	InfoFunc              func(ctx context.Context) (map[string]interface{}, error)
	StartFunc             func(ctx context.Context) error
	StopFunc              func(ctx context.Context) error
	CloseFunc             func() error
	ScanFunc              func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error)
	RangeFunc             func(ctx context.Context) (int, error)
	BoundsFunc            func(ctx context.Context) (image.Point, error)
	AngularResolutionFunc func(ctx context.Context) (float64, error)
}

func (ld *LidarDevice) Info(ctx context.Context) (map[string]interface{}, error) {
	if ld.InfoFunc == nil {
		return ld.Device.Info(ctx)
	}
	return ld.InfoFunc(ctx)
}

func (ld *LidarDevice) Start(ctx context.Context) error {
	if ld.StartFunc == nil {
		return ld.Device.Start(ctx)
	}
	return ld.StartFunc(ctx)
}

func (ld *LidarDevice) Stop(ctx context.Context) error {
	if ld.StopFunc == nil {
		return ld.Device.Stop(ctx)
	}
	return ld.StopFunc(ctx)
}

func (ld *LidarDevice) Close() error {
	if ld.CloseFunc == nil {
		return utils.TryClose(ld.Device)
	}
	return ld.CloseFunc()
}

func (ld *LidarDevice) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	if ld.ScanFunc == nil {
		return ld.Device.Scan(ctx, options)
	}
	return ld.ScanFunc(ctx, options)
}

func (ld *LidarDevice) Range(ctx context.Context) (int, error) {
	if ld.RangeFunc == nil {
		return ld.Device.Range(ctx)
	}
	return ld.RangeFunc(ctx)
}

func (ld *LidarDevice) Bounds(ctx context.Context) (image.Point, error) {
	if ld.BoundsFunc == nil {
		return ld.Device.Bounds(ctx)
	}
	return ld.BoundsFunc(ctx)
}

func (ld *LidarDevice) AngularResolution(ctx context.Context) (float64, error) {
	if ld.AngularResolutionFunc == nil {
		return ld.Device.AngularResolution(ctx)
	}
	return ld.AngularResolutionFunc(ctx)
}

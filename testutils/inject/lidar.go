package inject

import (
	"context"
	"sync"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/utils"

	"github.com/golang/geo/r2"
)

type Lidar struct {
	sync.Mutex
	lidar.Device
	InfoFunc              func(ctx context.Context) (map[string]interface{}, error)
	StartFunc             func(ctx context.Context) error
	StopFunc              func(ctx context.Context) error
	CloseFunc             func() error
	ScanFunc              func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error)
	RangeFunc             func(ctx context.Context) (float64, error)
	BoundsFunc            func(ctx context.Context) (r2.Point, error)
	AngularResolutionFunc func(ctx context.Context) (float64, error)
}

func (ld *Lidar) Info(ctx context.Context) (map[string]interface{}, error) {
	if ld.InfoFunc == nil {
		return ld.Device.Info(ctx)
	}
	return ld.InfoFunc(ctx)
}

func (ld *Lidar) Start(ctx context.Context) error {
	if ld.StartFunc == nil {
		return ld.Device.Start(ctx)
	}
	return ld.StartFunc(ctx)
}

func (ld *Lidar) Stop(ctx context.Context) error {
	if ld.StopFunc == nil {
		return ld.Device.Stop(ctx)
	}
	return ld.StopFunc(ctx)
}

func (ld *Lidar) Close() error {
	if ld.CloseFunc == nil {
		return utils.TryClose(ld.Device)
	}
	return ld.CloseFunc()
}

func (ld *Lidar) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	ld.Lock()
	scanFunc := ld.ScanFunc
	ld.Unlock()
	if scanFunc == nil {
		return ld.Device.Scan(ctx, options)
	}
	return scanFunc(ctx, options)
}

func (ld *Lidar) Range(ctx context.Context) (float64, error) {
	if ld.RangeFunc == nil {
		return ld.Device.Range(ctx)
	}
	return ld.RangeFunc(ctx)
}

func (ld *Lidar) Bounds(ctx context.Context) (r2.Point, error) {
	if ld.BoundsFunc == nil {
		return ld.Device.Bounds(ctx)
	}
	return ld.BoundsFunc(ctx)
}

func (ld *Lidar) AngularResolution(ctx context.Context) (float64, error) {
	if ld.AngularResolutionFunc == nil {
		return ld.Device.AngularResolution(ctx)
	}
	return ld.AngularResolutionFunc(ctx)
}

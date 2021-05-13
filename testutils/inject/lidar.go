package inject

import (
	"context"
	"sync"

	"go.viam.com/core/lidar"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r2"
)

// Lidar is an injected lidar.
type Lidar struct {
	sync.Mutex
	lidar.Lidar
	InfoFunc              func(ctx context.Context) (map[string]interface{}, error)
	StartFunc             func(ctx context.Context) error
	StopFunc              func(ctx context.Context) error
	CloseFunc             func() error
	ScanFunc              func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error)
	RangeFunc             func(ctx context.Context) (float64, error)
	BoundsFunc            func(ctx context.Context) (r2.Point, error)
	AngularResolutionFunc func(ctx context.Context) (float64, error)
}

// Info calls the injected Info or the real version.
func (ld *Lidar) Info(ctx context.Context) (map[string]interface{}, error) {
	if ld.InfoFunc == nil {
		return ld.Lidar.Info(ctx)
	}
	return ld.InfoFunc(ctx)
}

// Start calls the injected Start or the real version.
func (ld *Lidar) Start(ctx context.Context) error {
	if ld.StartFunc == nil {
		return ld.Lidar.Start(ctx)
	}
	return ld.StartFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (ld *Lidar) Stop(ctx context.Context) error {
	if ld.StopFunc == nil {
		return ld.Lidar.Stop(ctx)
	}
	return ld.StopFunc(ctx)
}

// Close calls the injected Close or the real version.
func (ld *Lidar) Close() error {
	if ld.CloseFunc == nil {
		return utils.TryClose(ld.Lidar)
	}
	return ld.CloseFunc()
}

// Scan calls the injected Scan or the real version.
func (ld *Lidar) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	ld.Lock()
	scanFunc := ld.ScanFunc
	ld.Unlock()
	if scanFunc == nil {
		return ld.Lidar.Scan(ctx, options)
	}
	return scanFunc(ctx, options)
}

// Range calls the injected Range or the real version.
func (ld *Lidar) Range(ctx context.Context) (float64, error) {
	if ld.RangeFunc == nil {
		return ld.Lidar.Range(ctx)
	}
	return ld.RangeFunc(ctx)
}

// Bounds calls the injected Bounds or the real version.
func (ld *Lidar) Bounds(ctx context.Context) (r2.Point, error) {
	if ld.BoundsFunc == nil {
		return ld.Lidar.Bounds(ctx)
	}
	return ld.BoundsFunc(ctx)
}

// AngularResolution calls the injected AngularResolution or the real version.
func (ld *Lidar) AngularResolution(ctx context.Context) (float64, error) {
	if ld.AngularResolutionFunc == nil {
		return ld.Lidar.AngularResolution(ctx)
	}
	return ld.AngularResolutionFunc(ctx)
}

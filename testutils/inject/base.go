package inject

import (
	"context"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Base is an injected base.
type Base struct {
	base.Base
	name             resource.Name
	NameFunc         func() resource.Name
	DoFunc           func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	MoveStraightFunc func(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error
	SpinFunc         func(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error
	StopFunc         func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc     func(context.Context) (bool, error)
	CloseFunc        func(ctx context.Context) error
	SetPowerFunc     func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error
	SetVelocityFunc  func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error
	PropertiesFunc   func(ctx context.Context, extra map[string]interface{}) (base.Properties, error)
	GeometriesFunc   func(ctx context.Context) ([]spatialmath.Geometry, error)
}

// NewBase returns a new injected base.
func NewBase(name string) *Base {
	return &Base{name: base.Named(name)}
}

// Name returns the name of the resource.
func (b *Base) Name() resource.Name {
	if b.NameFunc == nil {
		return b.name
	}
	return b.NameFunc()
}

// MoveStraight calls the injected MoveStraight or the real version.
func (b *Base) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	if b.MoveStraightFunc == nil {
		return b.Base.MoveStraight(ctx, distanceMm, mmPerSec, extra)
	}
	return b.MoveStraightFunc(ctx, distanceMm, mmPerSec, extra)
}

// Spin calls the injected Spin or the real version.
func (b *Base) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	if b.SpinFunc == nil {
		return b.Base.Spin(ctx, angleDeg, degsPerSec, extra)
	}
	return b.SpinFunc(ctx, angleDeg, degsPerSec, extra)
}

// Stop calls the injected Stop or the real version.
func (b *Base) Stop(ctx context.Context, extra map[string]interface{}) error {
	if b.StopFunc == nil {
		return b.Base.Stop(ctx, extra)
	}
	return b.StopFunc(ctx, extra)
}

// IsMoving calls the injected IsMoving or the real version.
func (b *Base) IsMoving(ctx context.Context) (bool, error) {
	if b.IsMovingFunc == nil {
		return b.Base.IsMoving(ctx)
	}
	return b.IsMovingFunc(ctx)
}

// Close calls the injected Close or the real version.
func (b *Base) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		if b.Base == nil {
			return nil
		}
		return b.Base.Close(ctx)
	}
	return b.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (b *Base) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return b.Base.DoCommand(ctx, cmd)
	}
	return b.DoFunc(ctx, cmd)
}

// SetPower calls the injected SetPower or the real version.
func (b *Base) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	if b.SetPowerFunc == nil {
		return b.Base.SetPower(ctx, linear, angular, extra)
	}
	return b.SetPowerFunc(ctx, linear, angular, extra)
}

// SetVelocity calls the injected SetVelocity or the real version.
func (b *Base) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	if b.SetVelocityFunc == nil {
		return b.Base.SetVelocity(ctx, linear, angular, extra)
	}
	return b.SetVelocityFunc(ctx, linear, angular, extra)
}

// Properties returns the base's properties.
func (b *Base) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	if b.PropertiesFunc == nil {
		return b.Base.Properties(ctx, extra)
	}
	return b.PropertiesFunc(ctx, extra)
}

// Geometries returns the base's geometries.
func (b *Base) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	if b.GeometriesFunc == nil {
		return b.Base.Geometries(ctx, extra)
	}
	return b.GeometriesFunc(ctx)
}

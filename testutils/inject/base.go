package inject

import (
	"context"

	"github.com/golang/geo/r3"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Base is an injected base.
type Base struct {
	base.Base
	name             resource.Name
	DoFunc           func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc       func(ctx context.Context) (map[string]interface{}, error)
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
	return b.name
}

// MoveStraight calls the injected MoveStraight or the real version.
func (b *Base) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	if b.MoveStraightFunc == nil {
		return errtrace.Wrap(b.Base.MoveStraight(ctx, distanceMm, mmPerSec, extra))
	}
	return errtrace.Wrap(b.MoveStraightFunc(ctx, distanceMm, mmPerSec, extra))
}

// Spin calls the injected Spin or the real version.
func (b *Base) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	if b.SpinFunc == nil {
		return errtrace.Wrap(b.Base.Spin(ctx, angleDeg, degsPerSec, extra))
	}
	return errtrace.Wrap(b.SpinFunc(ctx, angleDeg, degsPerSec, extra))
}

// Stop calls the injected Stop or the real version.
func (b *Base) Stop(ctx context.Context, extra map[string]interface{}) error {
	if b.StopFunc == nil {
		return errtrace.Wrap(b.Base.Stop(ctx, extra))
	}
	return errtrace.Wrap(b.StopFunc(ctx, extra))
}

// IsMoving calls the injected IsMoving or the real version.
func (b *Base) IsMoving(ctx context.Context) (bool, error) {
	if b.IsMovingFunc == nil {
		return errtrace.Wrap2(b.Base.IsMoving(ctx))
	}
	return errtrace.Wrap2(b.IsMovingFunc(ctx))
}

// Close calls the injected Close or the real version.
func (b *Base) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		if b.Base == nil {
			return nil
		}
		return errtrace.Wrap(b.Base.Close(ctx))
	}
	return errtrace.Wrap(b.CloseFunc(ctx))
}

// DoCommand calls the injected DoCommand or the real version.
func (b *Base) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return errtrace.Wrap2(b.Base.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(b.DoFunc(ctx, cmd))
}

// SetPower calls the injected SetPower or the real version.
func (b *Base) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	if b.SetPowerFunc == nil {
		return errtrace.Wrap(b.Base.SetPower(ctx, linear, angular, extra))
	}
	return errtrace.Wrap(b.SetPowerFunc(ctx, linear, angular, extra))
}

// SetVelocity calls the injected SetVelocity or the real version.
func (b *Base) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	if b.SetVelocityFunc == nil {
		return errtrace.Wrap(b.Base.SetVelocity(ctx, linear, angular, extra))
	}
	return errtrace.Wrap(b.SetVelocityFunc(ctx, linear, angular, extra))
}

// Properties returns the base's properties.
func (b *Base) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	if b.PropertiesFunc == nil {
		return errtrace.Wrap2(b.Base.Properties(ctx, extra))
	}
	return errtrace.Wrap2(b.PropertiesFunc(ctx, extra))
}

// Geometries returns the base's geometries.
func (b *Base) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	if b.GeometriesFunc == nil {
		return errtrace.Wrap2(b.Base.Geometries(ctx, extra))
	}
	return errtrace.Wrap2(b.GeometriesFunc(ctx))
}

// Status calls the injected Status or the real version.
func (b *Base) Status(ctx context.Context) (map[string]interface{}, error) {
	if b.StatusFunc != nil {
		return errtrace.Wrap2(b.StatusFunc(ctx))
	}
	if b.Base != nil {
		return errtrace.Wrap2(b.Base.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

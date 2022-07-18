// Package fake implements a fake base.
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		base.Subtype,
		"fake",
		registry.Component{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return &Base{Name: config.Name}, nil
			},
		},
	)
}

var _ = base.LocalBase(&Base{})

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	generic.Echo
	Name       string
	CloseCount int
}

// MoveStraight does nothing.
func (b *Base) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	return nil
}

// Spin does nothing.
func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	return nil
}

// SetPower does nothing.
func (b *Base) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	return nil
}

// SetVelocity does nothing.
func (b *Base) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	return nil
}

// GetWidth returns some arbitrary width.
func (b *Base) GetWidth(ctx context.Context) (int, error) {
	return 600, nil
}

// Stop does nothing.
func (b *Base) Stop(ctx context.Context) error {
	return nil
}

// IsMoving always returns false.
func (b *Base) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// Close does nothing.
func (b *Base) Close() {
	b.CloseCount++
}

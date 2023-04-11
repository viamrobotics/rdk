// Package fake implements a fake base.
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

func init() {
	registry.RegisterComponent(
		base.Subtype,
		resource.NewDefaultModel("fake"),
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
func (b *Base) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	return nil
}

// Spin does nothing.
func (b *Base) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	return nil
}

// SetPower does nothing.
func (b *Base) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return nil
}

// SetVelocity does nothing.
func (b *Base) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return nil
}

// Width returns some arbitrary width.
func (b *Base) Width(ctx context.Context) (int, error) {
	return 600, nil
}

// Stop does nothing.
func (b *Base) Stop(ctx context.Context, extra map[string]interface{}) error {
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

// // WrapWithKinematics allows a fake base to satisfy the KinematicWrappable interface
// func (b *Base) WrapWithKinematics(ctx context.Context, slamName slam.Service) (base.KinematicBase, error) {
// 	return b, nil
// }

// // CurrentInputs is necessary for a fake base to be a KinematicBase
// func (b *Base) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
// 	return referenceframe.FloatsToInputs([]float64{0, 0}), nil
// }

// // CurrentInputs is necessary for a fake base to be a KinematicBase
// func (b *Base) GoToInputs(context.Context, []referenceframe.Input) error {
// 	return nil
// }

// // ModelFrame is necessary for a fake base to be a KinematicBase

package inject

import (
	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/spatialmath"
)

// MovementSensor is an injected MovementSensor.
type MovementSensor struct {
	movementsensor.MovementSensor
	PositionFunc        func(ctx context.Context) (*geo.Point, float64, error)
	LinearVelocityFunc  func(ctx context.Context) (r3.Vector, error)
	AngularVelocityFunc func(ctx context.Context) (spatialmath.AngularVelocity, error)
	CompassHeadingFunc  func(ctx context.Context) (float64, error)
	OrientationFunc     func(ctx context.Context) (spatialmath.Orientation, error)

	DoFunc    func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// Close calls the injected Close or the real version.
func (i *MovementSensor) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.MovementSensor)
	}
	return i.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (i *MovementSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.DoFunc == nil {
		return i.MovementSensor.DoCommand(ctx, cmd)
	}
	return i.DoFunc(ctx, cmd)
}

// Position func or passthrough.
func (i *MovementSensor) Position(ctx context.Context) (*geo.Point, float64, error) {
	if i.PositionFunc == nil {
		return i.MovementSensor.Position(ctx)
	}
	return i.PositionFunc(ctx)
}

// LinearVelocity func or passthrough.
func (i *MovementSensor) LinearVelocity(ctx context.Context) (r3.Vector, error) {
	if i.PositionFunc == nil {
		return i.MovementSensor.LinearVelocity(ctx)
	}
	return i.LinearVelocityFunc(ctx)
}

// AngularVelocity func or passthrough.
func (i *MovementSensor) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	if i.PositionFunc == nil {
		return i.MovementSensor.AngularVelocity(ctx)
	}
	return i.AngularVelocityFunc(ctx)
}

// Orientation func or passthrough.
func (i *MovementSensor) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	if i.PositionFunc == nil {
		return i.MovementSensor.Orientation(ctx)
	}
	return i.OrientationFunc(ctx)
}

// CompassHeading func or passthrough.
func (i *MovementSensor) CompassHeading(ctx context.Context) (float64, error) {
	if i.PositionFunc == nil {
		return i.MovementSensor.CompassHeading(ctx)
	}
	return i.CompassHeadingFunc(ctx)
}

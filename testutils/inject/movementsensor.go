package inject

import (
	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/spatialmath"
)

// MovementSensor is an injected MovementSensor.
type MovementSensor struct {
	movementsensor.MovementSensor
	GetPositionFunc        func(ctx context.Context) (*geo.Point, float64, error)
	GetLinearVelocityFunc  func(ctx context.Context) (r3.Vector, error)
	GetAngularVelocityFunc func(ctx context.Context) (spatialmath.AngularVelocity, error)
	GetCompassHeadingFunc  func(ctx context.Context) (float64, error)
	GetOrientationFunc     func(ctx context.Context) (spatialmath.Orientation, error)

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

// GetPosition func or passthrough.
func (i *MovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	if i.GetPositionFunc == nil {
		return i.MovementSensor.GetPosition(ctx)
	}
	return i.GetPositionFunc(ctx)
}

// GetLinearVelocity func or passthrough.
func (i *MovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	if i.GetPositionFunc == nil {
		return i.MovementSensor.GetLinearVelocity(ctx)
	}
	return i.GetLinearVelocityFunc(ctx)
}

// GetAngularVelocity func or passthrough.
func (i *MovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	if i.GetPositionFunc == nil {
		return i.MovementSensor.GetAngularVelocity(ctx)
	}
	return i.GetAngularVelocityFunc(ctx)
}

// GetOrientation func or passthrough.
func (i *MovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	if i.GetPositionFunc == nil {
		return i.MovementSensor.GetOrientation(ctx)
	}
	return i.GetOrientationFunc(ctx)
}

// GetCompassHeading func or passthrough.
func (i *MovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	if i.GetPositionFunc == nil {
		return i.MovementSensor.GetCompassHeading(ctx)
	}
	return i.GetCompassHeadingFunc(ctx)
}

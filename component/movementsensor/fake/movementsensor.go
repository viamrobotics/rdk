// Package fake is a fake MovementSensor for testing
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return movementsensor.MovementSensor(&fakeMovementSensor{}), nil
		}})
}

type fakeMovementSensor struct{}

func (f *fakeMovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, float64, error) {
	p := &geo.Point{}
	return p, 0, 0, nil
}

func (f *fakeMovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (f *fakeMovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (f *fakeMovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	return 0, nil
}

func (f *fakeMovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

func (f *fakeMovementSensor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (f *fakeMovementSensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	return movementsensor.GetReadings(ctx, f)
}

func (f *fakeMovementSensor) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
		OrientationSupported:     true,
		PositionSupported:        true,
		CompassHeadingSupported:  true,
	}, nil
}

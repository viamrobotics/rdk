// Package fake is a fake MovementSensor for testing
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
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

func (f *fakeMovementSensor) Position(ctx context.Context) (*geo.Point, float64, error) {
	p := geo.NewPoint(40.7, -73.98)
	return p, 0, nil
}

func (f *fakeMovementSensor) LinearVelocity(ctx context.Context) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (f *fakeMovementSensor) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (f *fakeMovementSensor) CompassHeading(ctx context.Context) (float64, error) {
	return 0, nil
}

func (f *fakeMovementSensor) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

func (f *fakeMovementSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (f *fakeMovementSensor) Accuracy(ctx context.Context) (map[string]float32, error) {
	return map[string]float32{}, nil
}

func (f *fakeMovementSensor) Readings(ctx context.Context) (map[string]interface{}, error) {
	return movementsensor.Readings(ctx, f)
}

func (f *fakeMovementSensor) Properties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
		OrientationSupported:     true,
		PositionSupported:        true,
		CompassHeadingSupported:  true,
	}, nil
}

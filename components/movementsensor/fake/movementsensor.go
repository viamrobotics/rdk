// Package fake is a fake MovementSensor for testing
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var model = resource.DefaultModelFamily.WithModel("fake")

// Config is used for converting fake movementsensor attributes.
type Config struct {
	resource.TriviallyValidateConfig
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, *Config]{Constructor: NewMovementSensor})
}

// NewMovementSensor makes a new fake movement sensor.
func NewMovementSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	return &MovementSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}, nil
}

// MovementSensor implements is a fake movement sensor interface.
type MovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	logger golog.Logger
}

// Position gets the position of a fake movementsensor.
func (f *MovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	p := geo.NewPoint(40.7, -73.98)
	return p, 50.5, nil
}

// LinearVelocity gets the linear velocity of a fake movementsensor.
func (f *MovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{Y: 5.4}, nil
}

// LinearAcceleration gets the linear acceleration of a fake movementsensor.
func (f *MovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{X: 2.2, Y: 4.5, Z: 2}, nil
}

// AngularVelocity gets the angular velocity of a fake movementsensor.
func (f *MovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{Z: 1}, nil
}

// CompassHeading gets the compass headings of a fake movementsensor.
func (f *MovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 25, nil
}

// Orientation gets the orientation of a fake movementsensor.
func (f *MovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

// DoCommand uses a map string to run custom functionality of a fake movementsensor.
func (f *MovementSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// Accuracy gets the accuracy of a fake movementsensor.
func (f *MovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, nil
}

// Readings gets the readings of a fake movementsensor.
func (f *MovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return movementsensor.Readings(ctx, f, extra)
}

// Properties returns the properties of a fake movementsensor.
func (f *MovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:     true,
		AngularVelocitySupported:    true,
		OrientationSupported:        true,
		PositionSupported:           true,
		CompassHeadingSupported:     true,
		LinearAccelerationSupported: true,
	}, nil
}

// Start returns the fix of a fake gps movementsensor.
func (f *MovementSensor) Start(ctx context.Context) error { return nil }

// Close returns the fix of a fake gps movementsensor.
func (f *MovementSensor) Close(ctx context.Context) error {
	return nil
}

// ReadFix returns the fix of a fake gps movementsensor.
func (f *MovementSensor) ReadFix(ctx context.Context) (int, error) { return 1, nil }

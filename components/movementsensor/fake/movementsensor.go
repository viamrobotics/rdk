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

const modelname = "fake"

type AttrConfig struct {
	ConnectionType string `json:"connection_type,omitempty"`
}

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return movementsensor.MovementSensor(&MovementSensor{}), nil
		}})

	config.RegisterComponentAttributeMapConverter(movementsensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

type MovementSensor struct {
	CancelCtx context.Context
	Logger    golog.Logger
}

func (f *MovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	p := geo.NewPoint(40.7, -73.98)
	return p, 50.5, nil
}

func (f *MovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	return r3.Vector{Y: 5.4}, nil
}

func (f *MovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{Z: 1}, nil
}

func (f *MovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	return 25, nil
}

func (f *MovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

func (f *MovementSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (f *MovementSensor) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	return map[string]float32{}, nil
}

func (f *MovementSensor) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	return movementsensor.GetReadings(ctx, f)
}

func (f *MovementSensor) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
		OrientationSupported:     true,
		PositionSupported:        true,
		CompassHeadingSupported:  true,
	}, nil
}

func (f *MovementSensor) Start(ctx context.Context) error          { return nil }
func (f *MovementSensor) Close() error                             { return nil }
func (f *MovementSensor) ReadFix(ctx context.Context) (int, error) { return 1, nil }

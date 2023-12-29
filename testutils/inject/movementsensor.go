package inject

import (
	"context"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// MovementSensor is an injected MovementSensor.
type MovementSensor struct {
	movementsensor.MovementSensor
	name                        resource.Name
	Mu                          sync.RWMutex
	PositionFuncExtraCap        map[string]interface{}
	PositionFunc                func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error)
	LinearVelocityFuncExtraCap  map[string]interface{}
	LinearVelocityFunc          func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)
	AngularVelocityFuncExtraCap map[string]interface{}
	AngularVelocityFunc         func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error)
	CompassHeadingFuncExtraCap  map[string]interface{}
	CompassHeadingFunc          func(ctx context.Context, extra map[string]interface{}) (float64, error)
	LinearAccelerationExtraCap  map[string]interface{}
	LinearAccelerationFunc      func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)
	OrientationFuncExtraCap     map[string]interface{}
	OrientationFunc             func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error)
	PropertiesFuncExtraCap      map[string]interface{}
	PropertiesFunc              func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error)
	AccuracyFuncExtraCap        map[string]interface{}
	AccuracyFunc                func(ctx context.Context, extra map[string]interface{}) (map[string]float32, float32, float32,
		movementsensor.NmeaGGAFixType, float32, error)
	ReadingsFuncExtraCap map[string]interface{}
	ReadingsFunc         func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
	DoFunc               func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc            func() error
}

// NewMovementSensor returns a new injected movement sensor.
func NewMovementSensor(name string) *MovementSensor {
	return &MovementSensor{name: movementsensor.Named(name)}
}

// Name returns the name of the resource.
func (i *MovementSensor) Name() resource.Name {
	return i.name
}

// Close calls the injected Close or the real version.
func (i *MovementSensor) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		if i.MovementSensor == nil {
			return nil
		}
		return i.MovementSensor.Close(ctx)
	}
	return i.CloseFunc()
}

// DoCommand calls the injected DoCommand or the real version.
func (i *MovementSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.DoFunc == nil {
		return i.MovementSensor.DoCommand(ctx, cmd)
	}
	return i.DoFunc(ctx, cmd)
}

// Position func or passthrough.
func (i *MovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	i.Mu.Lock()
	defer i.Mu.Unlock()
	if i.PositionFunc == nil {
		return i.MovementSensor.Position(ctx, extra)
	}
	i.PositionFuncExtraCap = extra
	return i.PositionFunc(ctx, extra)
}

// LinearVelocity func or passthrough.
func (i *MovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	if i.LinearVelocityFunc == nil {
		return i.MovementSensor.LinearVelocity(ctx, extra)
	}
	i.LinearVelocityFuncExtraCap = extra
	return i.LinearVelocityFunc(ctx, extra)
}

// AngularVelocity func or passthrough.
func (i *MovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	if i.AngularVelocityFunc == nil {
		return i.MovementSensor.AngularVelocity(ctx, extra)
	}
	i.AngularVelocityFuncExtraCap = extra
	return i.AngularVelocityFunc(ctx, extra)
}

// LinearAcceleration func or passthrough.
func (i *MovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	if i.LinearAccelerationFunc == nil {
		return i.MovementSensor.LinearAcceleration(ctx, extra)
	}
	i.LinearAccelerationExtraCap = extra
	return i.LinearAccelerationFunc(ctx, extra)
}

// Orientation func or passthrough.
func (i *MovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	if i.OrientationFunc == nil {
		return i.MovementSensor.Orientation(ctx, extra)
	}
	i.OrientationFuncExtraCap = extra
	return i.OrientationFunc(ctx, extra)
}

// CompassHeading func or passthrough.
func (i *MovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	if i.CompassHeadingFunc == nil {
		return i.MovementSensor.CompassHeading(ctx, extra)
	}
	i.CompassHeadingFuncExtraCap = extra
	return i.CompassHeadingFunc(ctx, extra)
}

// Properties func or passthrough.
func (i *MovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	if i.PropertiesFunc == nil {
		return i.MovementSensor.Properties(ctx, extra)
	}
	i.PropertiesFuncExtraCap = extra
	return i.PropertiesFunc(ctx, extra)
}

// Accuracy func or passthrough.
func (i *MovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32,
	float32, float32, movementsensor.NmeaGGAFixType, float32, error) {
	if i.AccuracyFunc == nil {
		return i.MovementSensor.Accuracy(ctx, extra)
	}
	i.AccuracyFuncExtraCap = extra
	return i.AccuracyFunc(ctx, extra)
}

// Readings func or passthrough.
func (i *MovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if i.ReadingsFunc == nil {
		return i.MovementSensor.Readings(ctx, extra)
	}
	i.ReadingsFuncExtraCap = extra
	return i.ReadingsFunc(ctx, extra)
}

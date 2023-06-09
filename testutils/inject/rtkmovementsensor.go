package inject

import (
	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

type RTKMovementSensor struct {
	name resource.Name

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
	AccuracyFunc                func(ctx context.Context, extra map[string]interface{}) (map[string]float32, error)
	ConnectFunc                 func(casterAddr string, user string, pwd string, maxAttempts int) error
	NtripStatusFunc             func() (bool, error)
	GetStreamFunc               func(mountPoint string, maxAttempts int) error
	ReadFixFunc                 func(ctx context.Context) (int, error)

	DoFunc    func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func() error
}

// NewMovementSensor returns a new injected movement sensor.
func NewRTKMovementSensor(name string) *RTKMovementSensor {
	return &RTKMovementSensor{name: movementsensor.Named(name)}
}

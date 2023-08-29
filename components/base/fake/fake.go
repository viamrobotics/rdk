// Package fake implements a fake base.
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterComponent(
		base.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[base.Base, resource.NoNativeConfig]{Constructor: NewBase},
	)
}

const (
	defaultWidthMm               = 600
	defaultMinimumTurningRadiusM = 0
	defaultWheelCircumferenceM   = 3
)

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	resource.Named
	resource.TriviallyReconfigurable
	CloseCount               int
	WidthMeters              float64
	TurningRadius            float64
	WheelCircumferenceMeters float64
	Geometry                 []spatialmath.Geometry
	logger                   golog.Logger
}

// NewBase instantiates a new base of the fake model type.
func NewBase(_ context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (base.Base, error) {
	b := &Base{
		Named:    conf.ResourceName().AsNamed(),
		Geometry: []spatialmath.Geometry{},
		logger:   logger,
	}
	if conf.Frame != nil && conf.Frame.Geometry != nil {
		geometry, err := conf.Frame.Geometry.ParseConfig()
		if err != nil {
			return nil, err
		}
		b.Geometry = []spatialmath.Geometry{geometry}
	}
	b.WidthMeters = defaultWidthMm * 0.001
	b.TurningRadius = defaultMinimumTurningRadiusM
	return b, nil
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

// Stop does nothing.
func (b *Base) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving always returns false.
func (b *Base) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// Close does nothing.
func (b *Base) Close(ctx context.Context) error {
	b.CloseCount++
	return nil
}

// Properties returns the base's properties.
func (b *Base) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	return base.Properties{
		TurningRadiusMeters:      b.TurningRadius,
		WidthMeters:              b.WidthMeters,
		WheelCircumferenceMeters: b.WheelCircumferenceMeters,
	}, nil
}

// Geometries returns the geometries associated with the fake base.
func (b *Base) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return b.Geometry, nil
}

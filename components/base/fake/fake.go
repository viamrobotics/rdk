// Package fake implements a fake base.
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
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
)

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	resource.Named
	resource.TriviallyReconfigurable
	CloseCount int
	Geometry   []spatialmath.Geometry
}

// NewBase instantiates a new base of the fake model type.
func NewBase(_ context.Context, _ resource.Dependencies, conf resource.Config, _ golog.Logger) (base.Base, error) {
	// TODO(RSDK-3316): read the config to get the geometries and set them here
	return &Base{
		Named: conf.ResourceName().AsNamed(),
	}, nil
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
		TurningRadiusMeters: defaultMinimumTurningRadiusM,
		WidthMeters:         defaultWidthMm * 0.001, // convert to meters
	}, nil
}

// Geometries returns the geometries associated with the fake base.
func (b *Base) Geometries(ctx context.Context) ([]spatialmath.Geometry, error) {
	return b.Geometry, nil
}

type KinematicBase struct {
	*Base
	model     referenceframe.Model
	localizer motion.Localizer
	inputs    []referenceframe.Input
}

// WrapWithKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func (b *Base) WrapWithKinematics(
	ctx context.Context,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
) (*KinematicBase, error) {
	var geometry spatialmath.Geometry
	if b.Geometry != nil {
		geometry = b.Geometry[0]
	}
	model, err := referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}
	return &KinematicBase{
		Base:      b,
		model:     model,
		localizer: localizer,
		inputs:    make([]referenceframe.Input, len(model.DoF())),
	}, nil
}

func (kb *KinematicBase) ModelFrame() referenceframe.Model {
	return kb.model
}

func (kb *KinematicBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return kb.inputs, nil
}

func (kb *KinematicBase) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	_, err := kb.model.Transform(inputs)
	kb.inputs = inputs
	return err
}

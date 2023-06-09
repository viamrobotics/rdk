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
	defaultMinimumTurningRadiusM = 0.8
)

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	resource.Named
	resource.TriviallyReconfigurable
	CloseCount int
	geometry   *referenceframe.LinkConfig
}

// NewBase instantiates a new base of the fake model type.
func NewBase(ctx context.Context, _ resource.Dependencies, conf resource.Config, _ golog.Logger) (base.Base, error) {
	return &Base{
		Named:    conf.ResourceName().AsNamed(),
		geometry: conf.Frame,
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

type kinematicBase struct {
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
) (base.KinematicBase, error) {
	geometry, err := base.CollisionGeometry(b.geometry)
	if err != nil {
		return nil, err
	}
	model, err := referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}

	initialPose, err := localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	initialPoint := initialPose.Pose().Point()

	return &kinematicBase{
		Base:      b,
		model:     model,
		localizer: localizer,
		inputs:    referenceframe.FloatsToInputs([]float64{initialPoint.X, initialPoint.Y, initialPose.Pose().Orientation().OrientationVectorRadians().Theta}),
	}, nil
}

func (kb *kinematicBase) ModelFrame() referenceframe.Model {
	return kb.model
}

func (kb *kinematicBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return kb.inputs, nil
}

func (kb *kinematicBase) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	_, err := kb.model.Transform(inputs)
	kb.inputs = inputs
	return err
}

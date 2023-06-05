// Package fake implements a fake base.
package fake

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion/localizer"
)

func init() {
	resource.RegisterComponent(
		base.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[base.Base, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (base.Base, error) {
				return NewBase(ctx, conf)
			},
		},
	)
}

const defaultWidth = 600

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	resource.Named
	resource.TriviallyReconfigurable
	CloseCount int
	geometry   *referenceframe.LinkConfig
}

// NewBase instantiates a new base of the fake model type.
func NewBase(ctx context.Context, conf resource.Config) (base.LocalBase, error) {
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

// Width returns some arbitrary width.
func (b *Base) Width(ctx context.Context) (int, error) {
	return defaultWidth, nil
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

type kinematicBase struct {
	*Base
	model referenceframe.Model
	localizer.Localizer
	inputs []referenceframe.Input
}

// WrapWithKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func (b *Base) WrapWithKinematics(
	ctx context.Context,
	local localizer.Localizer,
	limits []referenceframe.Limit) (base.KinematicBase, error) {
	slamSvc, ok := local.(localizer.SLAMLocalizer)
	if ok {
		geometry, err := base.CollisionGeometry(b.geometry)
		if err != nil {
			return nil, err
		}
		model, err := referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
		if err != nil {
			return nil, err
		}
		fs := referenceframe.NewEmptyFrameSystem("")
		if err := fs.AddFrame(model, fs.World()); err != nil {
			return nil, err
		}
		return &kinematicBase{
			Base:      b,
			model:     model,
			Localizer: slamSvc,
			inputs:    make([]referenceframe.Input, len(model.DoF())),
		}, err
	}

	movementSensor, ok := local.(localizer.MovementSensorLocalizer)
	if ok {
		geometry, err := base.CollisionGeometry(b.geometry)
		if err != nil {
			return nil, err
		}
		model, err := referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
		if err != nil {
			return nil, err
		}
		fs := referenceframe.NewEmptyFrameSystem("")
		if err := fs.AddFrame(model, fs.World()); err != nil {
			return nil, err
		}
		return &kinematicBase{
			Base:      b,
			model:     model,
			Localizer: movementSensor,
			inputs:    make([]referenceframe.Input, len(model.DoF())),
		}, err
	}
	return nil, errors.New("TODO: write an error")
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

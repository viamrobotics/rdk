package kinematicbase

import (
	"context"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

type fakeKinematics struct {
	*fake.Base
	model     referenceframe.Model
	localizer motion.Localizer
	inputs    []referenceframe.Input
}

// WrapWithFakeKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func WrapWithFakeKinematics(
	ctx context.Context,
	b *fake.Base,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
) (KinematicBase, error) {
	var geometry spatialmath.Geometry
	if b.Geometry != nil {
		geometry = b.Geometry[0]
	}
	model, err := referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}
	
	return &fakeKinematics{
		Base:      b,
		model:     model,
		localizer: localizer,
		inputs: make([]referenceframe.Input, len(model.DoF())),
	}, nil
}

func (fk *fakeKinematics) ModelFrame() referenceframe.Model {
	return fk.model
}

func (fk *fakeKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return fk.inputs, nil
}

func (fk *fakeKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	_, err := fk.model.Transform(inputs)
	fk.inputs = inputs
	return err
}

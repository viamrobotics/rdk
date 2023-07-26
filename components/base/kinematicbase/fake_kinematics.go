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
	model     referenceframe.Frame
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
	position, err := localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := position.Pose().Point()
	fk := &fakeKinematics{
		Base:      b,
		localizer: localizer,
		inputs:    []referenceframe.Input{{pt.X}, {pt.Y}, {0}},
	}
	var geometry spatialmath.Geometry
	if fk.Base.Geometry != nil {
		geometry = fk.Base.Geometry[0]
	}
	fk.model, err = referenceframe.New2DMobileModelFrame(fk.Base.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}
	return fk, nil
}

func (fk *fakeKinematics) Kinematics() referenceframe.Frame {
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

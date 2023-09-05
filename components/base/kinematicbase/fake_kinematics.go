package kinematicbase

import (
	"context"
	"sync"
	"time"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

type fakeKinematics struct {
	*fake.Base
	planningFrame, executionFrame referenceframe.Frame
	localizer                     motion.Localizer
	inputs                        []referenceframe.Input
	options                       Options
	lock                          sync.Mutex
}

// WrapWithFakeKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func WrapWithFakeKinematics(
	ctx context.Context,
	b *fake.Base,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	options Options,
) (KinematicBase, error) {
	position, err := localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := position.Pose().Point()
	fk := &fakeKinematics{
		Base:      b,
		localizer: localizer,
		inputs:    []referenceframe.Input{{pt.X}, {pt.Y}},
	}
	var geometry spatialmath.Geometry
	if len(fk.Base.Geometry) != 0 {
		geometry = fk.Base.Geometry[0]
	}

	fk.executionFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}

	if options.PositionOnlyMode {
		fk.planningFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits[:2], geometry)
		if err != nil {
			return nil, err
		}
	} else {
		fk.planningFrame = fk.executionFrame
	}

	fk.options = options
	return fk, nil
}

func (fk *fakeKinematics) Kinematics() referenceframe.Frame {
	return fk.planningFrame
}

func (fk *fakeKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	fk.lock.Lock()
	defer fk.lock.Unlock()
	return fk.inputs, nil
}

func (fk *fakeKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	_, err := fk.planningFrame.Transform(inputs)
	if err != nil {
		return err
	}
	fk.lock.Lock()
	fk.inputs = inputs
	fk.lock.Unlock()

	// Sleep for a short amount to time to simulate a base taking some amount of time to reach the inputs
	time.Sleep(150 * time.Millisecond)
	return nil
}

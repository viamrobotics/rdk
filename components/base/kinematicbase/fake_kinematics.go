package kinematicbase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

type fakeKinematics struct {
	*fake.Base
	origin                        *referenceframe.PoseInFrame
	planningFrame, executionFrame referenceframe.Frame
	inputs                        []referenceframe.Input
	options                       Options
	sensorNoise                   spatialmath.Pose
	lock                          sync.Mutex
}

// WrapWithFakeKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func WrapWithFakeKinematics(
	ctx context.Context,
	b *fake.Base,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	options Options,
	sensorNoise spatialmath.Pose,
) (KinematicBase, error) {
	position, err := localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := position.Pose().Point()
	if sensorNoise == nil {
		sensorNoise = spatialmath.NewZeroPose()
	}
	fk := &fakeKinematics{
		Base:        b,
		origin:      position,
		inputs:      []referenceframe.Input{{pt.X}, {pt.Y}},
		sensorNoise: sensorNoise,
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

func (fk *fakeKinematics) ErrorState(
	ctx context.Context,
	plan [][]referenceframe.Input,
	currentNode int,
) (spatialmath.Pose, error) {
	fmt.Println("the index ", currentNode)
	current, err := fk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	fmt.Println("the current ", current.Pose().Point())
	desiredPose, err := fk.planningFrame.Transform(plan[currentNode])
	if err != nil {
		return nil, err
	}
	return spatialmath.PoseBetween(current.Pose(), desiredPose), nil
}

func (fk *fakeKinematics) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	fk.lock.Lock()
	inputs := fk.inputs
	fk.lock.Unlock()
	currentPose, err := fk.planningFrame.Transform(inputs)
	if err != nil {
		return nil, err
	}
	return referenceframe.NewPoseInFrame(fk.origin.Parent(), spatialmath.Compose(currentPose, fk.sensorNoise)), nil
}

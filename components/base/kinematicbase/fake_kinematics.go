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
	parentFrame                   string
	planningFrame, executionFrame referenceframe.Frame
	inputs                        []referenceframe.Input
	options                       Options
	sensorNoise                   spatialmath.Pose
	lock                          sync.Mutex
	fs                            referenceframe.FrameSystem
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
		parentFrame: position.Parent(),
		inputs:      referenceframe.FloatsToInputs([]float64{pt.X, pt.Y}),
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

	fs := referenceframe.NewEmptyFrameSystem("fakeKinematics")
	if err = fs.AddFrame(fk.planningFrame, fs.World()); err != nil {
		return nil, err
	}
	fk.fs = fs
	return fk, nil
}

func (fk *fakeKinematics) FrameSystem() referenceframe.FrameSystem {
	return fk.fs
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
	current, err := fk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
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
	return referenceframe.NewPoseInFrame(fk.parentFrame, spatialmath.Compose(currentPose, fk.sensorNoise)), nil
}

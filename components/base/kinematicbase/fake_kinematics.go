//go:build !no_cgo

package kinematicbase

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

type fakeDiffDriveKinematics struct {
	*fake.Base
	parentFrame                   string
	planningFrame, executionFrame referenceframe.Frame
	inputs                        []referenceframe.Input
	options                       Options
	sensorNoise                   spatialmath.Pose
	lock                          sync.RWMutex
}

// WrapWithFakeDiffDriveKinematics creates a DiffDrive KinematicBase from the fake Base so that it satisfies the ModelFramer and
// InputEnabled interfaces.
func WrapWithFakeDiffDriveKinematics(
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
	fk := &fakeDiffDriveKinematics{
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
	return fk, nil
}

func (fk *fakeDiffDriveKinematics) Kinematics() referenceframe.Frame {
	return fk.planningFrame
}

func (fk *fakeDiffDriveKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fk.lock.RLock()
	defer fk.lock.RUnlock()
	return fk.inputs, nil
}

func (fk *fakeDiffDriveKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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

func (fk *fakeDiffDriveKinematics) ErrorState(
	ctx context.Context,
	plan [][]referenceframe.Input,
	currentNode int,
) (spatialmath.Pose, error) {
	return fk.sensorNoise, nil
}

func (fk *fakeDiffDriveKinematics) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fk.lock.RLock()
	inputs := fk.inputs
	fk.lock.RUnlock()
	currentPose, err := fk.planningFrame.Transform(inputs)
	if err != nil {
		return nil, err
	}
	return referenceframe.NewPoseInFrame(fk.parentFrame, spatialmath.Compose(currentPose, fk.sensorNoise)), nil
}

type fakePTGKinematics struct {
	*fake.Base
	frame       referenceframe.Frame
	options     Options
	sensorNoise spatialmath.Pose
	origin      *referenceframe.PoseInFrame
	lock        sync.RWMutex
	logger      logging.Logger
	sleepTime   time.Duration
}

type NewPTGFrameFromKinematicOptionsReq struct {
	Base    *fake.Base
	Origin  *referenceframe.PoseInFrame
	Options Options
}

func NewPTGFrameFromKinematicOptions(
	ctx context.Context,
	req NewPTGFrameFromKinematicOptionsReq,
	logger logging.Logger,
) (referenceframe.Frame, error) {
	properties, err := req.Base.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	baseMillimetersPerSecond := req.Options.LinearVelocityMMPerSec
	if baseMillimetersPerSecond == 0 {
		baseMillimetersPerSecond = defaultLinearVelocityMMPerSec
	}

	baseTurningRadiusMeters := properties.TurningRadiusMeters
	if baseTurningRadiusMeters < 0 {
		return nil, errors.New("can only wrap with PTG kinematics if turning radius is greater than or equal to zero")
	}

	geometries, err := req.Base.Geometries(ctx, nil)
	if err != nil {
		return nil, err
	}

	return tpspace.NewPTGFrameFromKinematicOptions(
		req.Base.Name().ShortName(),
		logger,
		baseMillimetersPerSecond,
		req.Options.AngularVelocityDegsPerSec,
		baseTurningRadiusMeters,
		req.Options.MaxMoveStraightMM, // If zero, will use default on the receiver end.
		0,                             // If zero, will use default on the receiver end.
		geometries,
		req.Options.NoSkidSteer,
	)
}

type WrapWithFakePTGKinematicsReq struct {
	Base          *fake.Base
	Origin        *referenceframe.PoseInFrame
	Options       Options
	SensorNoise   spatialmath.Pose
	OverRideFrame referenceframe.Frame
	SleepTime     time.Duration
}

// WrapWithFakePTGKinematics creates a PTG KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled
// interfaces.
func WrapWithFakePTGKinematics(
	ctx context.Context,
	req WrapWithFakePTGKinematicsReq,
	logger logging.Logger,
) (KinematicBase, error) {
	frame := req.OverRideFrame
	if frame == nil {
		f, err := NewPTGFrameFromKinematicOptions(ctx, NewPTGFrameFromKinematicOptionsReq{
			Base:    req.Base,
			Origin:  req.Origin,
			Options: req.Options,
		}, logger)
		if err != nil {
			return nil, err
		}
		frame = f
	}

	sensorNoise := req.SensorNoise
	if sensorNoise == nil {
		sensorNoise = spatialmath.NewZeroPose()
	}
	fk := &fakePTGKinematics{
		Base:        req.Base,
		frame:       frame,
		origin:      req.Origin,
		sensorNoise: sensorNoise,
		logger:      logger,
		sleepTime:   req.SleepTime,
	}

	fk.options = req.Options
	return fk, nil
}

func (fk *fakePTGKinematics) Kinematics() referenceframe.Frame {
	return fk.frame
}

func (fk *fakePTGKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return make([]referenceframe.Input, 3), nil
}

func (fk *fakePTGKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	newPose, err := fk.frame.Transform(inputs)
	if err != nil {
		return err
	}

	fk.lock.Lock()
	fk.origin = referenceframe.NewPoseInFrame(fk.origin.Parent(), spatialmath.Compose(fk.origin.Pose(), newPose))
	fk.lock.Unlock()
	timeoutCtx, cancelFn := context.WithTimeout(ctx, fk.sleepTime)
	defer cancelFn()
	<-timeoutCtx.Done()
	return ctx.Err()
}

func (fk *fakePTGKinematics) ErrorState(
	ctx context.Context,
	plan [][]referenceframe.Input,
	currentNode int,
) (spatialmath.Pose, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return fk.sensorNoise, nil
}

func (fk *fakePTGKinematics) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fk.lock.RLock()
	defer fk.lock.RUnlock()
	origin := fk.origin
	return referenceframe.NewPoseInFrame(origin.Parent(), spatialmath.Compose(origin.Pose(), fk.sensorNoise)), nil
}

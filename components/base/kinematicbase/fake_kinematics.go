//go:build !no_cgo

package kinematicbase

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
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
	fk.lock.RLock()
	defer fk.lock.RUnlock()
	return fk.inputs, nil
}

func (fk *fakeDiffDriveKinematics) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, inputs := range inputSteps {
		_, err := fk.planningFrame.Transform(inputs)
		if err != nil {
			return err
		}
		fk.lock.Lock()
		fk.inputs = inputs
		fk.lock.Unlock()

		// Sleep for a short amount to time to simulate a base taking some amount of time to reach the inputs
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func (fk *fakeDiffDriveKinematics) ErrorState(ctx context.Context, plan motionplan.Plan, currentNode int) (spatialmath.Pose, error) {
	return fk.sensorNoise, nil
}

func (fk *fakeDiffDriveKinematics) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
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
	localizer    motion.Localizer
	frame        referenceframe.Frame
	options      Options
	sensorNoise  spatialmath.Pose
	ptgs         []tpspace.PTGSolver
	currentInput []referenceframe.Input
	origin       *referenceframe.PoseInFrame
	positionlock sync.RWMutex
	inputLock    sync.RWMutex
	logger       logging.Logger
	sleepTime    int
}

// WrapWithFakePTGKinematics creates a PTG KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled
// interfaces.
func WrapWithFakePTGKinematics(
	ctx context.Context,
	b *fake.Base,
	logger logging.Logger,
	origin *referenceframe.PoseInFrame,
	options Options,
	sensorNoise spatialmath.Pose,
	sleepTime int,
) (KinematicBase, error) {
	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	baseMillimetersPerSecond := options.LinearVelocityMMPerSec
	if baseMillimetersPerSecond == 0 {
		baseMillimetersPerSecond = defaultLinearVelocityMMPerSec
	}

	baseTurningRadiusMeters := properties.TurningRadiusMeters
	if baseTurningRadiusMeters < 0 {
		return nil, errors.New("can only wrap with PTG kinematics if turning radius is greater than or equal to zero")
	}

	angVelocityDegsPerSecond, err := correctAngularVelocityWithTurnRadius(
		logger,
		baseTurningRadiusMeters,
		baseMillimetersPerSecond,
		options.AngularVelocityDegsPerSec,
	)
	if err != nil {
		return nil, err
	}

	geometries, err := b.Geometries(ctx, nil)
	if err != nil {
		return nil, err
	}

	nonzeroBaseTurningRadiusMeters := (baseMillimetersPerSecond / rdkutils.DegToRad(angVelocityDegsPerSecond)) / 1000.

	frame, err := tpspace.NewPTGFrameFromKinematicOptions(
		b.Name().ShortName(),
		logger,
		nonzeroBaseTurningRadiusMeters,
		0, // If zero, will use default on the receiver end.
		geometries,
		options.NoSkidSteer,
		baseTurningRadiusMeters == 0,
	)
	if err != nil {
		return nil, err
	}

	if sensorNoise == nil {
		sensorNoise = spatialmath.NewZeroPose()
	}

	ptgProv, ok := frame.(tpspace.PTGProvider)
	if !ok {
		return nil, errors.New("unable to cast ptgk frame to a PTG Provider")
	}
	ptgs := ptgProv.PTGSolvers()

	fk := &fakePTGKinematics{
		Base:         b,
		frame:        frame,
		origin:       origin,
		ptgs:         ptgs,
		currentInput: zeroInput,
		sensorNoise:  sensorNoise,
		logger:       logger,
		sleepTime:    sleepTime,
	}
	initLocalizer := &fakePTGKinematicsLocalizer{fk}
	fk.localizer = motion.TwoDLocalizer(initLocalizer)

	fk.options = options
	return fk, nil
}

func (fk *fakePTGKinematics) Kinematics() referenceframe.Frame {
	return fk.frame
}

func (fk *fakePTGKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	fk.inputLock.RLock()
	defer fk.inputLock.RUnlock()
	return fk.currentInput, nil
}

func (fk *fakePTGKinematics) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	defer func() {
		fk.inputLock.Lock()
		fk.currentInput = zeroInput
		fk.inputLock.Unlock()
	}()

	for _, inputs := range inputSteps {
		fk.positionlock.RLock()
		startingPose := fk.origin
		fk.positionlock.RUnlock()

		fk.inputLock.Lock()
		fk.currentInput = []referenceframe.Input{inputs[0], inputs[1], {Value: 0}}
		fk.inputLock.Unlock()

		finalPose, err := fk.frame.Transform(inputs)
		if err != nil {
			return err
		}

		steps := motionplan.PathStepCount(spatialmath.NewZeroPose(), finalPose, 2)
		startCfg := referenceframe.FloatsToInputs([]float64{inputs[0].Value, inputs[1].Value, 0})
		var interpolatedConfigurations [][]referenceframe.Input
		for i := 0; i <= steps; i++ {
			interp := float64(i) / float64(steps)
			interpConfig := referenceframe.InterpolateInputs(startCfg, inputs, interp)
			interpolatedConfigurations = append(interpolatedConfigurations, interpConfig)
		}
		for _, inter := range interpolatedConfigurations {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			relativePose, err := fk.frame.Transform(inter)
			if err != nil {
				return err
			}
			newPose := spatialmath.Compose(startingPose.Pose(), relativePose)

			fk.positionlock.Lock()
			fk.origin = referenceframe.NewPoseInFrame(fk.origin.Parent(), newPose)
			fk.positionlock.Unlock()

			fk.inputLock.Lock()
			fk.currentInput = []referenceframe.Input{inputs[0], inputs[1], inter[2]}
			fk.inputLock.Unlock()

			time.Sleep(time.Duration(fk.sleepTime) * time.Microsecond * 10)
		}
	}
	return nil
}

func (fk *fakePTGKinematics) ErrorState(ctx context.Context, plan motionplan.Plan, currentNode int) (spatialmath.Pose, error) {
	return fk.sensorNoise, nil
}

func (fk *fakePTGKinematics) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	return fk.localizer.CurrentPosition(ctx)
}

type fakePTGKinematicsLocalizer struct {
	fk *fakePTGKinematics
}

func (fkl *fakePTGKinematicsLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	fkl.fk.positionlock.RLock()
	defer fkl.fk.positionlock.RUnlock()
	origin := fkl.fk.origin
	return referenceframe.NewPoseInFrame(origin.Parent(), spatialmath.Compose(origin.Pose(), fkl.fk.sensorNoise)), nil
}

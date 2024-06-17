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
	parentFrame                      string
	planningFrame, localizationFrame referenceframe.Frame
	inputs                           []referenceframe.Input
	options                          Options
	sensorNoise                      spatialmath.Pose
	lock                             sync.RWMutex
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

	fk.localizationFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}

	if options.PositionOnlyMode {
		fk.planningFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits[:2], geometry)
		if err != nil {
			return nil, err
		}
	} else {
		fk.planningFrame = fk.localizationFrame
	}

	fk.options = options
	return fk, nil
}

func (fk *fakeDiffDriveKinematics) Kinematics() referenceframe.Frame {
	return fk.planningFrame
}

func (fk *fakeDiffDriveKinematics) LocalizationFrame() referenceframe.Frame {
	return fk.localizationFrame
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

func (fk *fakeDiffDriveKinematics) ErrorState(ctx context.Context) (spatialmath.Pose, error) {
	return fk.sensorNoise, nil
}

func (fk *fakeDiffDriveKinematics) ExecutionState(ctx context.Context) (motionplan.ExecutionState, error) {
	return motionplan.ExecutionState{}, errors.New("fakeDiffDriveKinematics does not support executionState")
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
	localizer                        motion.Localizer
	planningFrame, localizationFrame referenceframe.Frame
	options                          Options
	sensorNoise                      spatialmath.Pose
	ptgs                             []tpspace.PTGSolver
	currentInput                     []referenceframe.Input
	currentIndex                     int
	plan                             motionplan.Plan
	origin                           *referenceframe.PoseInFrame
	positionlock                     sync.RWMutex
	inputLock                        sync.RWMutex
	logger                           logging.Logger
	sleepTime                        int
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

	// construct planning frame
	planningFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
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

	// construct localization frame
	localizationFrame, err := referenceframe.NewPoseFrame(b.Name().ShortName()+"LocalizationFrame", nil)
	if err != nil {
		return nil, err
	}

	if sensorNoise == nil {
		sensorNoise = spatialmath.NewZeroPose()
	}

	ptgProv, ok := planningFrame.(tpspace.PTGProvider)
	if !ok {
		return nil, errors.New("unable to cast ptgk frame to a PTG Provider")
	}
	ptgs := ptgProv.PTGSolvers()
	traj := motionplan.Trajectory{{planningFrame.Name(): zeroInput}}
	path := motionplan.Path{
		{planningFrame.Name(): referenceframe.NewPoseInFrame(origin.Parent(), spatialmath.Compose(origin.Pose(), sensorNoise))},
	}
	zeroPlan := motionplan.NewSimplePlan(path, traj)

	fk := &fakePTGKinematics{
		Base:              b,
		planningFrame:     planningFrame,
		localizationFrame: localizationFrame,
		origin:            origin,
		ptgs:              ptgs,
		currentInput:      zeroInput,
		currentIndex:      0,
		plan:              zeroPlan,
		sensorNoise:       sensorNoise,
		logger:            logger,
		sleepTime:         sleepTime,
	}
	initLocalizer := &fakePTGKinematicsLocalizer{fk}
	fk.localizer = motion.TwoDLocalizer(initLocalizer)

	fk.options = options
	return fk, nil
}

func (fk *fakePTGKinematics) Kinematics() referenceframe.Frame {
	return fk.planningFrame
}

func (fk *fakePTGKinematics) LocalizationFrame() referenceframe.Frame {
	return fk.localizationFrame
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
		fk.currentIndex = 0

		traj := motionplan.Trajectory{{fk.planningFrame.Name(): zeroInput}}
		path := motionplan.Path{
			{fk.planningFrame.Name(): referenceframe.NewPoseInFrame(fk.origin.Parent(), spatialmath.Compose(fk.origin.Pose(), fk.sensorNoise))},
		}
		fk.plan = motionplan.NewSimplePlan(path, traj)
		fk.inputLock.Unlock()
	}()

	currPos, err := fk.CurrentPosition(ctx)
	if err != nil {
		return err
	}

	fk.inputLock.Lock()
	fk.plan, err = inputsToPlan(inputSteps, currPos, fk.Kinematics())
	fk.inputLock.Unlock()
	if err != nil {
		return err
	}

	for i, inputs := range inputSteps {
		fk.positionlock.RLock()
		startingPose := fk.origin
		fk.positionlock.RUnlock()

		fk.inputLock.Lock()
		fk.currentIndex = i
		fk.currentInput, err = fk.planningFrame.Interpolate(zeroInput, inputs, 0)
		fk.inputLock.Unlock()
		if err != nil {
			return err
		}
		finalPose, err := fk.planningFrame.Transform(inputs)
		if err != nil {
			return err
		}

		steps := motionplan.PathStepCount(spatialmath.NewZeroPose(), finalPose, 2)
		var interpolatedConfigurations [][]referenceframe.Input
		for i := 0; i <= steps; i++ {
			interp := float64(i) / float64(steps)
			interpConfig, err := fk.planningFrame.Interpolate(zeroInput, inputs, interp)
			if err != nil {
				return err
			}
			interpolatedConfigurations = append(interpolatedConfigurations, interpConfig)
		}
		for _, inter := range interpolatedConfigurations {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			relativePose, err := fk.planningFrame.Transform(inter)
			if err != nil {
				return err
			}
			newPose := spatialmath.Compose(startingPose.Pose(), relativePose)

			fk.positionlock.Lock()
			fk.origin = referenceframe.NewPoseInFrame(fk.origin.Parent(), newPose)
			fk.positionlock.Unlock()

			fk.inputLock.Lock()
			fk.currentInput = inter
			fk.inputLock.Unlock()

			time.Sleep(time.Duration(fk.sleepTime) * time.Microsecond * 10)
		}
	}
	return nil
}

func (fk *fakePTGKinematics) ErrorState(ctx context.Context) (spatialmath.Pose, error) {
	return fk.sensorNoise, nil
}

func (fk *fakePTGKinematics) ExecutionState(ctx context.Context) (motionplan.ExecutionState, error) {
	fk.inputLock.RLock()
	defer fk.inputLock.RUnlock()
	pos, err := fk.CurrentPosition(ctx)
	if err != nil {
		return motionplan.ExecutionState{}, err
	}

	return motionplan.NewExecutionState(
		fk.plan,
		fk.currentIndex,
		map[string][]referenceframe.Input{fk.Kinematics().Name(): fk.currentInput},
		map[string]*referenceframe.PoseInFrame{fk.LocalizationFrame().Name(): pos},
	)
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

func inputsToPlan(
	inputs [][]referenceframe.Input,
	startPose *referenceframe.PoseInFrame,
	frame referenceframe.Frame,
) (motionplan.Plan, error) {
	runningPose := startPose.Pose()
	traj := motionplan.Trajectory{}
	path := motionplan.Path{}
	for _, input := range inputs {
		inputPose, err := frame.Transform(input)
		if err != nil {
			return nil, err
		}
		runningPose = spatialmath.Compose(runningPose, inputPose)
		traj = append(traj, map[string][]referenceframe.Input{frame.Name(): input})
		path = append(path, map[string]*referenceframe.PoseInFrame{
			frame.Name(): referenceframe.NewPoseInFrame(startPose.Parent(), runningPose),
		})
	}

	return motionplan.NewSimplePlan(path, traj), nil
}

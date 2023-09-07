// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// Define a default speed to target for the base in the case where one is not provided.
const defaultBaseMMps = 600.

var zeroInput = make([]referenceframe.Input, 3)

const (
	ptgIndex int = iota
	trajectoryIndexWithinPTG
	distanceAlongTrajectoryIndex
)

type ptgBaseKinematics struct {
	base.Base
	motion.Localizer
	logger       golog.Logger
	frame        referenceframe.Frame
	fs           referenceframe.FrameSystem
	ptgs         []tpspace.PTG
	inputLock    sync.RWMutex
	currentInput []referenceframe.Input
}

// wrapWithPTGKinematics takes a Base component and adds a PTG kinematic model so that it can be controlled.
func wrapWithPTGKinematics(
	ctx context.Context,
	b base.Base,
	logger golog.Logger,
	localizer motion.Localizer,
	options Options,
) (KinematicBase, error) {
	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	baseMillimetersPerSecond := defaultBaseMMps
	if options.LinearVelocityMMPerSec > 0 {
		baseMillimetersPerSecond = options.LinearVelocityMMPerSec
	}

	baseTurningRadius := properties.TurningRadiusMeters
	if options.AngularVelocityDegsPerSec > 0 {
		// Compute smallest allowable turning radius permitted by the given speeds. Use the greater of the two.
		calcTurnRadius := (baseMillimetersPerSecond / rdkutils.DegToRad(options.AngularVelocityDegsPerSec)) / 1000.
		baseTurningRadius = math.Max(baseTurningRadius, calcTurnRadius)
	}
	logger.Infof(
		"using baseMillimetersPerSecond %f and baseTurningRadius %f for PTG base kinematics",
		baseMillimetersPerSecond,
		baseTurningRadius,
	)

	if baseTurningRadius <= 0 {
		return nil, errors.New("can only wrap with PTG kinematics if turning radius is greater than zero")
	}

	geometries, err := b.Geometries(ctx, nil)
	if err != nil {
		return nil, err
	}

	frame, err := tpspace.NewPTGFrameFromTurningRadius(
		b.Name().ShortName(),
		logger,
		baseMillimetersPerSecond,
		baseTurningRadius,
		0, // pass 0 to use the default refDist
		geometries,
	)
	if err != nil {
		return nil, err
	}

	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(frame, fs.World()); err != nil {
		return nil, err
	}

	ptgProv, ok := frame.(tpspace.PTGProvider)
	if !ok {
		return nil, errors.New("unable to cast ptgk frame to a PTG Provider")
	}
	ptgs := ptgProv.PTGs()

	return &ptgBaseKinematics{
		Base:         b,
		Localizer:    localizer,
		logger:       logger,
		frame:        frame,
		fs:           fs,
		ptgs:         ptgs,
		currentInput: zeroInput,
	}, nil
}

func (ptgk *ptgBaseKinematics) Kinematics() referenceframe.Frame {
	return ptgk.frame
}

func (ptgk *ptgBaseKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// A PTG frame is always at its own origin, so current inputs are always all zero/not meaningful
	ptgk.inputLock.RLock()
	defer ptgk.inputLock.RUnlock()
	return ptgk.currentInput, nil
}

func (ptgk *ptgBaseKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) (err error) {
	if len(inputs) != 3 {
		return errors.New("inputs to ptg kinematic base must be length 3")
	}

	defer func() {
		ptgk.inputLock.Lock()
		ptgk.currentInput = zeroInput
		ptgk.inputLock.Unlock()
	}()

	ptgk.logger.Debugf("GoToInputs going to %v", inputs)

	selectedPTG := ptgk.ptgs[int(math.Round(inputs[ptgIndex].Value))]
	selectedTraj, err := selectedPTG.Trajectory(inputs[trajectoryIndexWithinPTG].Value, inputs[distanceAlongTrajectoryIndex].Value)
	if err != nil {
		return multierr.Combine(err, ptgk.Base.Stop(ctx, nil))
	}

	lastDist := 0.
	lastTime := 0.
	for _, trajNode := range selectedTraj {
		ptgk.inputLock.Lock() // In the case where there's actual contention here, this could cause timing issues; how to solve?
		ptgk.currentInput = []referenceframe.Input{inputs[0], inputs[1], {lastDist}}
		ptgk.inputLock.Unlock()
		lastDist = trajNode.Dist
		// TODO: Most trajectories update their velocities infrequently, or sometimes never.
		// This function could be improved by looking ahead through the trajectory and minimizing the amount of SetVelocity calls.
		timestep := time.Duration((trajNode.Time-lastTime)*1000*1000) * time.Microsecond
		lastTime = trajNode.Time
		linVel := r3.Vector{0, trajNode.LinVelMMPS, 0}
		angVel := r3.Vector{0, 0, rdkutils.RadToDeg(trajNode.AngVelRPS)}

		ptgk.logger.Debugf(
			"setting velocity to linear %v angular %v and running velocity step for %s",
			linVel,
			angVel,
			timestep,
		)

		err := ptgk.Base.SetVelocity(
			ctx,
			linVel,
			angVel,
			nil,
		)
		if err != nil {
			return multierr.Combine(err, ptgk.Base.Stop(ctx, nil))
		}
		utils.SelectContextOrWait(ctx, timestep)
	}

	return ptgk.Base.Stop(ctx, nil)
}

func (ptgk *ptgBaseKinematics) ErrorState(ctx context.Context, plan [][]referenceframe.Input, currentNode int) (spatialmath.Pose, error) {
	if currentNode < 0 || currentNode >= len(plan) {
		return nil, fmt.Errorf("cannot get ErrorState for node %d, must be >= 0 and less than plan length %d", currentNode, len(plan))
	}

	// Get pose-in-frame of the base via its localizer. The offset between the localizer and its base should already be accounted for.
	actualPIF, err := ptgk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}

	var nominalPose spatialmath.Pose

	// Determine the nominal pose, that is, the pose where the robot ought be if it had followed the plan perfectly up until this point.
	// This is done differently depending on what sort of frame we are working with.
	// TODO: The `rectifyTPspacePath` in motionplan does basically this. Deduplicate.
	runningPose := spatialmath.NewZeroPose()
	for i := 0; i < currentNode; i++ {
		wp := plan[i]
		wpPose, err := ptgk.frame.Transform(wp)
		if err != nil {
			return nil, err
		}
		runningPose = spatialmath.Compose(runningPose, wpPose)
	}

	// Determine how far through the current trajectory we are
	currentInputs, err := ptgk.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	currPose, err := ptgk.frame.Transform(currentInputs)
	if err != nil {
		return nil, err
	}
	nominalPose = spatialmath.Compose(runningPose, currPose)

	return spatialmath.PoseBetween(nominalPose, actualPIF.Pose()), nil
}

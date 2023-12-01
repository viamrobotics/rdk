//go:build !no_cgo

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

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var zeroInput = make([]referenceframe.Input, 3)

const (
	ptgIndex int = iota
	trajectoryIndexWithinPTG
	distanceAlongTrajectoryIndex
)

type ptgBaseKinematics struct {
	base.Base
	motion.Localizer
	logger       logging.Logger
	frame        referenceframe.Frame
	ptgs         []tpspace.PTGSolver
	inputLock    sync.RWMutex
	currentInput []referenceframe.Input
	origin       spatialmath.Pose
}

// wrapWithPTGKinematics takes a Base component and adds a PTG kinematic model so that it can be controlled.
func wrapWithPTGKinematics(
	ctx context.Context,
	b base.Base,
	logger logging.Logger,
	localizer motion.Localizer,
	options Options,
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

	logger.CInfof(ctx,
		"using baseMillimetersPerSecond %f and baseTurningRadius %f for PTG base kinematics",
		baseMillimetersPerSecond,
		baseTurningRadiusMeters,
	)

	if baseTurningRadiusMeters < 0 {
		return nil, errors.New("can only wrap with PTG kinematics if turning radius is greater than or equal to zero")
	}

	geometries, err := b.Geometries(ctx, nil)
	if err != nil {
		return nil, err
	}

	frame, err := tpspace.NewPTGFrameFromKinematicOptions(
		b.Name().ShortName(),
		logger,
		baseMillimetersPerSecond,
		options.AngularVelocityDegsPerSec,
		baseTurningRadiusMeters,
		options.MaxMoveStraightMM, // If zero, will use default on the receiver end.
		0,                         // If zero, will use default on the receiver end.
		geometries,
		options.NoSkidSteer,
	)
	if err != nil {
		return nil, err
	}

	ptgProv, ok := frame.(tpspace.PTGProvider)
	if !ok {
		return nil, errors.New("unable to cast ptgk frame to a PTG Provider")
	}
	ptgs := ptgProv.PTGSolvers()
	origin := spatialmath.NewZeroPose()
	if localizer != nil {
		originPIF, err := localizer.CurrentPosition(ctx)
		if err != nil {
			return nil, err
		}
		origin = originPIF.Pose()
	}

	return &ptgBaseKinematics{
		Base:         b,
		Localizer:    localizer,
		logger:       logger,
		frame:        frame,
		ptgs:         ptgs,
		currentInput: zeroInput,
		origin:       origin,
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

	ptgk.logger.CDebugf(ctx, "GoToInputs going to %v", inputs)

	selectedPTG := ptgk.ptgs[int(math.Round(inputs[ptgIndex].Value))]
	selectedTraj, err := selectedPTG.Trajectory(inputs[trajectoryIndexWithinPTG].Value, inputs[distanceAlongTrajectoryIndex].Value)
	if err != nil {
		stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFn()
		return multierr.Combine(err, ptgk.Base.Stop(stopCtx, nil))
	}

	lastDist := 0.
	lastTime := 0.
	lastLinVel := r3.Vector{}
	lastAngVel := r3.Vector{}
	for i, trajNode := range selectedTraj {
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

		// This should call SetVelocity if:
		// 1) this is the first iteration of the loop, or
		// 2) either of the linear or angular velocities has changed
		if i == 0 || !(linVel.ApproxEqual(lastLinVel) && angVel.ApproxEqual(lastAngVel)) {
			ptgk.logger.CDebugf(ctx,
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
				stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
				defer cancelFn()
				return multierr.Combine(err, ptgk.Base.Stop(stopCtx, nil))
			}
			lastLinVel = linVel
			lastAngVel = angVel
		}
		if !utils.SelectContextOrWait(ctx, timestep) {
			ptgk.logger.Debug(ctx.Err().Error())
			// context cancelled
			break
		}
	}

	stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFn()
	return ptgk.Base.Stop(stopCtx, nil)
}

func (ptgk *ptgBaseKinematics) ErrorState(ctx context.Context, plan [][]referenceframe.Input, currentNode int) (spatialmath.Pose, error) {
	if currentNode < 0 || currentNode >= len(plan) {
		return nil, fmt.Errorf("cannot get ErrorState for node %d, must be >= 0 and less than plan length %d", currentNode, len(plan))
	}

	// Get pose-in-frame of the base via its localizer. The offset between the localizer and its base should already be accounted for.
	actualPIFRaw, err := ptgk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	actualPIF := spatialmath.PoseBetween(ptgk.origin, actualPIFRaw.Pose())

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

	return spatialmath.PoseBetween(nominalPose, actualPIF), nil
}

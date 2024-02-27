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
	"go.viam.com/rdk/motionplan"
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

const (
	inputUpdateStep    = 0.1 // seconds
	stepDistResolution = 1.  // Before post-processing trajectory will have velocities every this many mm (or degs if spinning in place)
)

type ptgBaseKinematics struct {
	base.Base
	motion.Localizer
	logger logging.Logger
	frame  referenceframe.Frame
	ptgs   []tpspace.PTGSolver

	linVelocityMMPerSecond   float64
	angVelocityDegsPerSecond float64
	inputLock                sync.RWMutex
	currentInput             []referenceframe.Input
	origin                   spatialmath.Pose
	geometries               []spatialmath.Geometry
}

type arcStep struct {
	linVelMMps      r3.Vector
	angVelDegps     r3.Vector
	timestepSeconds float64
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

	linVelocityMMPerSecond := options.LinearVelocityMMPerSec
	if linVelocityMMPerSecond == 0 {
		linVelocityMMPerSecond = defaultLinearVelocityMMPerSec
	}

	// Update our angular velocity and our
	baseTurningRadiusMeters := properties.TurningRadiusMeters
	if baseTurningRadiusMeters < 0 {
		return nil, errors.New("can only wrap with PTG kinematics if turning radius is greater than or equal to zero")
	}

	angVelocityDegsPerSecond, err := correctAngularVelocityWithTurnRadius(
		logger,
		baseTurningRadiusMeters,
		linVelocityMMPerSecond,
		options.AngularVelocityDegsPerSec,
	)
	if err != nil {
		return nil, err
	}

	logger.CInfof(ctx,
		"using linVelocityMMPerSecond %f, angVelocityDegsPerSecond %f, and baseTurningRadiusMeters %f for PTG base kinematics",
		linVelocityMMPerSecond,
		angVelocityDegsPerSecond,
		baseTurningRadiusMeters,
	)

	geometries, err := b.Geometries(ctx, nil)
	if len(geometries) == 0 || err != nil {
		logger.CWarn(ctx, "base %s not configured with a geometry, will be considered a 300mm sphere for collision detection purposes.")
		sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 150., b.Name().Name)
		if err != nil {
			return nil, err
		}
		geometries = []spatialmath.Geometry{sphere}
	}

	nonzeroBaseTurningRadiusMeters := (linVelocityMMPerSecond / rdkutils.DegToRad(angVelocityDegsPerSecond)) / 1000.
	frame, err := tpspace.NewPTGFrameFromKinematicOptions(
		b.Name().ShortName(),
		logger,
		nonzeroBaseTurningRadiusMeters,
		0, // If zero, will use default trajectory count on the receiver end.
		geometries,
		options.NoSkidSteer,
		baseTurningRadiusMeters == 0,
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
		Base:                     b,
		Localizer:                localizer,
		logger:                   logger,
		frame:                    frame,
		ptgs:                     ptgs,
		linVelocityMMPerSecond:   linVelocityMMPerSecond,
		angVelocityDegsPerSecond: angVelocityDegsPerSecond,
		currentInput:             zeroInput,
		origin:                   origin,
		geometries:               geometries,
	}, nil
}

func (ptgk *ptgBaseKinematics) Kinematics() referenceframe.Frame {
	return ptgk.frame
}

func (ptgk *ptgBaseKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	ptgk.inputLock.RLock()
	defer ptgk.inputLock.RUnlock()
	return ptgk.currentInput, nil
}

func (ptgk *ptgBaseKinematics) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, inputs := range inputSteps {
		err := ptgk.goToInputs(ctx, inputs)
		if err != nil {
			return err
		}
	}

	stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFn()
	return ptgk.Base.Stop(stopCtx, nil)
}

func (ptgk *ptgBaseKinematics) goToInputs(ctx context.Context, inputs []referenceframe.Input) error {
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

	selectedTraj, err := selectedPTG.Trajectory(
		inputs[trajectoryIndexWithinPTG].Value,
		inputs[distanceAlongTrajectoryIndex].Value,
		stepDistResolution,
	)
	if err != nil {
		stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFn()
		return multierr.Combine(err, ptgk.Base.Stop(stopCtx, nil))
	}
	arcSteps := ptgk.trajectoryToArcSteps(selectedTraj)

	for _, step := range arcSteps {
		ptgk.inputLock.Lock() // In the case where there's actual contention here, this could cause timing issues; how to solve?
		ptgk.currentInput = []referenceframe.Input{inputs[0], inputs[1], {0}}
		ptgk.inputLock.Unlock()

		timestep := time.Duration(step.timestepSeconds*1000*1000) * time.Microsecond

		ptgk.logger.CDebugf(ctx,
			"setting velocity to linear %v angular %v and running velocity step for %s",
			step.linVelMMps,
			step.angVelDegps,
			timestep,
		)

		err := ptgk.Base.SetVelocity(
			ctx,
			step.linVelMMps,
			step.angVelDegps,
			nil,
		)
		if err != nil {
			stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
			defer cancelFn()
			return multierr.Combine(err, ptgk.Base.Stop(stopCtx, nil))
		}
		utils.PanicCapturingGo(func() {
			// We need to update currentInputs as we move through the arc.
			for timeElapsed := 0.; timeElapsed <= step.timestepSeconds; timeElapsed += inputUpdateStep {
				distIncVel := step.linVelMMps.Y
				if distIncVel == 0 {
					distIncVel = step.angVelDegps.Z
				}
				ptgk.inputLock.Lock()
				ptgk.currentInput = []referenceframe.Input{inputs[0], inputs[1], {math.Abs(distIncVel) * timeElapsed}}
				ptgk.inputLock.Unlock()
				utils.SelectContextOrWait(ctx, time.Duration(inputUpdateStep*1000*1000)*time.Microsecond)
			}
		})

		if !utils.SelectContextOrWait(ctx, timestep) {
			ptgk.logger.CDebug(ctx, ctx.Err().Error())
			// context cancelled
			break
		}
	}
	return nil
}

func (ptgk *ptgBaseKinematics) ErrorState(ctx context.Context, plan motionplan.Plan, currentNode int) (spatialmath.Pose, error) {
	traj := plan.Trajectory()
	if currentNode < 0 || traj == nil || currentNode >= len(traj) {
		return nil, fmt.Errorf("cannot get ErrorState for node %d, must be >= 0 and less than plan length %d", currentNode, len(traj))
	}
	waypoints, err := plan.Trajectory().GetFrameInputs(ptgk.Name().Name)
	if err != nil {
		return nil, err
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
	// TODO: We should be able to use the Path that exists in the plan rather than doing this duplicate work here
	runningPose := spatialmath.NewZeroPose()
	for i := 0; i < currentNode; i++ {
		wpPose, err := ptgk.frame.Transform(waypoints[i])
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

func (ptgk *ptgBaseKinematics) trajectoryToArcSteps(traj []*tpspace.TrajNode) []arcStep {
	finalSteps := []arcStep{}
	timeStep := 0.
	curDist := 0.
	// Trajectory distance is either length in mm, or if linear distance is not increasing, number of degrees to rotate in place.
	lastLinVel := r3.Vector{0, traj[0].LinVel * ptgk.linVelocityMMPerSecond, 0}
	lastAngVel := r3.Vector{0, 0, traj[0].AngVel * ptgk.angVelocityDegsPerSecond}
	nextStep := arcStep{
		linVelMMps:      lastLinVel,
		angVelDegps:     lastAngVel,
		timestepSeconds: 0,
	}
	for _, trajPt := range traj {
		nextLinVel := r3.Vector{0, trajPt.LinVel * ptgk.linVelocityMMPerSecond, 0}
		nextAngVel := r3.Vector{0, 0, trajPt.AngVel * ptgk.angVelocityDegsPerSecond}
		if nextStep.linVelMMps.Sub(nextLinVel).Norm2() > 1e-6 || nextStep.angVelDegps.Sub(nextAngVel).Norm2() > 1e-6 {
			// Changed velocity, make a new step
			nextStep.timestepSeconds = timeStep
			finalSteps = append(finalSteps, nextStep)
			nextStep = arcStep{
				linVelMMps:      nextLinVel,
				angVelDegps:     nextAngVel,
				timestepSeconds: 0,
			}
			timeStep = 0.
		}
		distIncrement := trajPt.Dist - curDist
		curDist += distIncrement
		if nextStep.linVelMMps.Y != 0 {
			timeStep += distIncrement / (math.Abs(nextStep.linVelMMps.Y))
		} else if nextStep.angVelDegps.Z != 0 {
			timeStep += distIncrement / (math.Abs(nextStep.angVelDegps.Z))
		}
	}
	nextStep.timestepSeconds = timeStep
	finalSteps = append(finalSteps, nextStep)
	return finalSteps
}

func correctAngularVelocityWithTurnRadius(logger logging.Logger, turnRadMeters, velocityMMps, angVelocityDegps float64) (float64, error) {
	angVelocityRadps := rdkutils.DegToRad(angVelocityDegps)
	turnRadMillimeters := turnRadMeters * 1000.
	if angVelocityRadps == 0 {
		if turnRadMeters == 0 {
			return -1, errors.New("cannot create ptg frame, turning radius and angular velocity cannot both be zero")
		}
		angVelocityRadps = velocityMMps / turnRadMillimeters
	} else if turnRadMeters > 0 {
		// Compute smallest allowable turning radius permitted by the given speeds. Use the greater of the two.
		calcTurnRadius := (velocityMMps / angVelocityRadps)
		if calcTurnRadius > turnRadMillimeters {
			// This is a debug message because the user will never notice the difference; the trajectories executed by the base will be a
			// subset of the ones that would have been had this conditional not been hit.
			logger.Debugf(
				"given turning radius was %f but a linear velocity of %f "+
					"meters per sec and angular velocity of %f degs per sec only allow a turning radius of %f, using that instead",
				turnRadMeters, velocityMMps/1000., angVelocityDegps, calcTurnRadius,
			)
		} else if calcTurnRadius < turnRadMillimeters {
			// If max allowed angular velocity would turn tighter than given turn radius, shrink the max used angular velocity
			// to match the requested tightest turn radius.
			angVelocityRadps = velocityMMps / turnRadMillimeters
			// This is a warning message because the user will observe the base turning at a different speed than the one requested.
			logger.Warnf(
				"given turning radius was %f but a linear velocity of %f "+
					"meters per sec and angular velocity of %f degs per sec would turn at a radius of %f. Decreasing angular velocity to %f.",
				turnRadMeters, velocityMMps/1000., angVelocityDegps, calcTurnRadius, rdkutils.RadToDeg(angVelocityRadps),
			)
		}
	}
	return rdkutils.RadToDeg(angVelocityRadps), nil
}

func (ptgk *ptgBaseKinematics) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return ptgk.geometries, nil
}

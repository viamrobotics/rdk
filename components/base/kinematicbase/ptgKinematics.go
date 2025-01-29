//go:build !no_cgo

// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"
	"sync"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var zeroInput = make([]referenceframe.Input, 4)

const (
	ptgIndex                          int = iota // The first input is the index of the associated PTG in the `ptgs` array
	trajectoryAlphaWithinPTG                     // The second input is the alpha value of the ptg to use
	startDistanceAlongTrajectoryIndex            // The third input is the start distance of the arc that will be executed
	endDistanceAlongTrajectoryIndex              // The fourth input is the end distance of the arc that will be executed
)

type ptgBaseKinematics struct {
	base.Base
	motion.Localizer
	logger                           logging.Logger
	planningFrame, localizationFrame referenceframe.Frame
	ptgs                             []tpspace.PTGSolver
	opts                             Options
	courseCorrectionIdx              int
	linVelocityMMPerSecond           float64
	angVelocityDegsPerSecond         float64
	nonzeroBaseTurningRadiusMeters   float64

	// All changeable state of the base is here
	inputLock    sync.RWMutex
	currentState baseState

	origin     spatialmath.Pose
	geometries []spatialmath.Geometry
	cancelFunc context.CancelFunc
}

type baseState struct {
	currentIdx            int
	currentInputs         []referenceframe.Input
	currentExecutingSteps []arcStep
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
		logger.CWarnf(
			ctx, "base %s not configured with a geometry, will be considered a 300mm sphere for collision detection purposes.",
			b.Name().Name,
		)
		sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 150., b.Name().Name)
		if err != nil {
			return nil, err
		}
		geometries = []spatialmath.Geometry{sphere}
	}

	nonzeroBaseTurningRadiusMeters := (linVelocityMMPerSecond / rdkutils.DegToRad(angVelocityDegsPerSecond)) / 1000.
	planningFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
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
	ptgProv, err := rdkutils.AssertType[tpspace.PTGProvider](planningFrame)
	if err != nil {
		return nil, err
	}
	ptgs := ptgProv.PTGSolvers()
	origin := spatialmath.NewZeroPose()

	ptgCourseCorrection, err := rdkutils.AssertType[tpspace.PTGCourseCorrection](planningFrame)
	if err != nil {
		return nil, err
	}
	courseCorrectionIdx := ptgCourseCorrection.CorrectionSolverIdx()

	if localizer != nil {
		originPIF, err := localizer.CurrentPosition(ctx)
		if err != nil {
			return nil, err
		}
		origin = originPIF.Pose()
	}
	startingState := baseState{currentInputs: zeroInput}

	localizationFrame, err := referenceframe.NewPoseFrame(b.Name().ShortName()+"_LocalizationFrame", nil)
	if err != nil {
		return nil, err
	}

	return &ptgBaseKinematics{
		Base:                           b,
		Localizer:                      localizer,
		logger:                         logger,
		planningFrame:                  planningFrame,
		localizationFrame:              localizationFrame,
		opts:                           options,
		ptgs:                           ptgs,
		courseCorrectionIdx:            courseCorrectionIdx,
		linVelocityMMPerSecond:         linVelocityMMPerSecond,
		angVelocityDegsPerSecond:       angVelocityDegsPerSecond,
		nonzeroBaseTurningRadiusMeters: nonzeroBaseTurningRadiusMeters,
		currentState:                   startingState,
		origin:                         origin,
		geometries:                     geometries,
	}, nil
}

func (ptgk *ptgBaseKinematics) Kinematics() referenceframe.Frame {
	return ptgk.planningFrame
}

func (ptgk *ptgBaseKinematics) LocalizationFrame() referenceframe.Frame {
	return ptgk.localizationFrame
}

// For a ptgBaseKinematics, `CurrentInputs` returns inputs which reflect what the base is currently doing.
// If the base is not moving, the CurrentInputs will all be zeros, and a `Transform()` will yield the zero pose.
// If the base is moving, then the inputs will be nonzero and the `Transform()` of the CurrentInputs will yield the pose at which the base
// is expected to arrive after completing execution of the current set of inputs.
func (ptgk *ptgBaseKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	ptgk.inputLock.RLock()
	defer ptgk.inputLock.RUnlock()

	return ptgk.currentState.currentInputs, nil
}

func (ptgk *ptgBaseKinematics) ExecutionState(ctx context.Context) (motionplan.ExecutionState, error) {
	if ptgk.Localizer == nil {
		return motionplan.ExecutionState{}, errors.New("cannot call ExecutionState on a base without a localizer")
	}

	actualPIF, err := ptgk.Localizer.CurrentPosition(ctx)
	if err != nil {
		return motionplan.ExecutionState{}, err
	}

	ptgk.inputLock.RLock()
	currentIdx := ptgk.currentState.currentIdx
	currentInputs := ptgk.currentState.currentInputs
	currentExecutingSteps := ptgk.currentState.currentExecutingSteps
	currentPlan := ptgk.stepsToPlan(currentExecutingSteps, actualPIF.Parent())
	ptgk.inputLock.RUnlock()

	return motionplan.NewExecutionState(
		currentPlan,
		currentIdx,
		referenceframe.FrameSystemInputs{ptgk.Kinematics().Name(): currentInputs},
		map[string]*referenceframe.PoseInFrame{ptgk.LocalizationFrame().Name(): actualPIF},
	)
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

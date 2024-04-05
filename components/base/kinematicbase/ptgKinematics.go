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
	logger                         logging.Logger
	frame                          referenceframe.Frame
	ptgs                           []tpspace.PTGSolver
	courseCorrectionIdx            int
	linVelocityMMPerSecond         float64
	angVelocityDegsPerSecond       float64
	nonzeroBaseTurningRadiusMeters float64

	inputLock             sync.RWMutex
	currentIdx            int
	currentInputs         []referenceframe.Input
	currentExecutingSteps []arcStep

	origin     spatialmath.Pose
	geometries []spatialmath.Geometry
	cancelFunc context.CancelFunc
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
	ptgProv, err := rdkutils.AssertType[tpspace.PTGProvider](frame)
	if err != nil {
		return nil, err
	}
	ptgs := ptgProv.PTGSolvers()
	origin := spatialmath.NewZeroPose()

	ptgCourseCorrection, err := rdkutils.AssertType[tpspace.PTGCourseCorrection](frame)
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

	return &ptgBaseKinematics{
		Base:                           b,
		Localizer:                      localizer,
		logger:                         logger,
		frame:                          frame,
		ptgs:                           ptgs,
		courseCorrectionIdx:            courseCorrectionIdx,
		linVelocityMMPerSecond:         linVelocityMMPerSecond,
		angVelocityDegsPerSecond:       angVelocityDegsPerSecond,
		nonzeroBaseTurningRadiusMeters: nonzeroBaseTurningRadiusMeters,
		currentInputs:                  zeroInput,
		origin:                         origin,
		geometries:                     geometries,
	}, nil
}

func (ptgk *ptgBaseKinematics) Kinematics() referenceframe.Frame {
	return ptgk.frame
}

func (ptgk *ptgBaseKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	ptgk.inputLock.RLock()
	defer ptgk.inputLock.RUnlock()
	return ptgk.currentInputs, nil
}

func (ptgk *ptgBaseKinematics) ErrorState(ctx context.Context) (spatialmath.Pose, error) {
	if ptgk.Localizer == nil {
		return nil, errors.New("cannot call ErrorState on a base without a localizer")
	}

	// Get pose-in-frame of the base via its localizer. The offset between the localizer and its base should already be accounted for.
	actualPIF, err := ptgk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}

	// Determine the nominal pose, that is, the pose where the robot ought be if it had followed the plan perfectly up until this point.
	ptgk.inputLock.RLock()
	currentIdx := ptgk.currentIdx
	currentExecutingSteps := ptgk.currentExecutingSteps
	ptgk.inputLock.RUnlock()
	currentInputs, err := ptgk.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}

	currPoseInArc, err := ptgk.frame.Transform(currentInputs)
	if err != nil {
		return nil, err
	}
	nominalPose := spatialmath.Compose(currentExecutingSteps[currentIdx].arcSegment.StartPosition, currPoseInArc)

	return spatialmath.PoseBetween(nominalPose, actualPIF.Pose()), nil
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

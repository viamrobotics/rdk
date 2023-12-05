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
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// The pause time when not using a localizer before moving on to next move step.
const defaultNoLocalizerDelay = 250 * time.Millisecond

var (
	// ErrMovementTimeout is used for when a movement call times out after no movement for some time.
	ErrMovementTimeout = errors.New("movement has timed out")
	// Input representation of origin.
	originInputs = []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}}
)

// wrapWithDifferentialDriveKinematics takes a wheeledBase component and adds a localizer to it
// It also adds kinematic model so that it can be controlled.
func wrapWithDifferentialDriveKinematics(
	ctx context.Context,
	b base.Base,
	logger logging.Logger,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	options Options,
) (KinematicBase, error) {
	ddk := &differentialDriveKinematics{
		Base:      b,
		Localizer: localizer,
		logger:    logger,
		options:   options,
		mutex:     sync.RWMutex{},
	}
	ddk.mutex.Lock()
	defer ddk.mutex.Unlock()

	geometries, err := b.Geometries(ctx, nil)
	if err != nil {
		return nil, err
	}
	// RSDK-4131 will update this so it is no longer necessary
	var geometry spatialmath.Geometry
	if len(geometries) > 1 {
		ddk.logger.Warn("multiple geometries specified for differential drive kinematic base, only can use the first at this time")
	}
	if len(geometries) > 0 {
		geometry = geometries[0]
	}
	sphere, err := spatialmath.BoundingSphere(geometry)
	if err != nil {
		logger.Warn("base %s not configured with a geometry, will be considered a point mass for collision detection purposes.")
		sphere = spatialmath.NewPoint(r3.Vector{}, b.Name().Name)
	}

	ddk.executionFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, sphere)
	if err != nil {
		return nil, err
	}

	if options.PositionOnlyMode {
		ddk.planningFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits[:2], sphere)
		if err != nil {
			return nil, err
		}
	} else {
		ddk.planningFrame = ddk.executionFrame
	}

	ddk.noLocalizerCacheInputs = originInputs
	return ddk, nil
}

type differentialDriveKinematics struct {
	base.Base
	motion.Localizer
	logger                        logging.Logger
	planningFrame, executionFrame referenceframe.Model
	options                       Options
	noLocalizerCacheInputs        []referenceframe.Input
	mutex                         sync.RWMutex
}

func (ddk *differentialDriveKinematics) Kinematics() referenceframe.Frame {
	return ddk.planningFrame
}

func (ddk *differentialDriveKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// If no localizer is present, CurrentInputs returns the expected position of the robot assuming after
	// each part of move command was completed accurately.
	if ddk.Localizer == nil {
		ddk.mutex.RLock()
		defer ddk.mutex.RUnlock()
		currentInputs := ddk.noLocalizerCacheInputs

		return currentInputs, nil
	}

	// TODO(rb): make a transformation from the component reference to the base frame
	pif, err := ddk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := pif.Pose().Point()
	// We should not have a problem with Gimbal lock by looking at yaw in the domain that most bases will be moving.
	// This could potentially be made more robust in the future, though.
	theta := math.Mod(pif.Pose().Orientation().EulerAngles().Yaw, 2*math.Pi)
	return []referenceframe.Input{{Value: pt.X}, {Value: pt.Y}, {Value: theta}}, nil
}

func (ddk *differentialDriveKinematics) GoToInputs(ctx context.Context, desired []referenceframe.Input) (err error) {
	// create capsule which defines the valid region for a base to be when driving to desired waypoint
	// deviationThreshold defines max distance base can be from path without error being thrown
	current, inputsErr := ddk.CurrentInputs(ctx)
	if inputsErr != nil {
		return inputsErr
	}
	validRegion, capsuleErr := ddk.newValidRegionCapsule(current, desired)
	if capsuleErr != nil {
		return capsuleErr
	}
	movementErr := make(chan error, 1)
	defer close(movementErr)

	cancelContext, cancel := context.WithCancel(ctx)
	defer cancel()

	if ddk.Localizer == nil {
		defer func() {
			ddk.mutex.Lock()
			defer ddk.mutex.Unlock()
			ddk.noLocalizerCacheInputs = originInputs
		}()
	}

	utils.PanicCapturingGo(func() {
		// this loop polls the error state and issues a corresponding command to move the base to the objective
		// when the base is within the positional threshold of the goal, exit the loop
		for err := cancelContext.Err(); err == nil; err = cancelContext.Err() {
			utils.SelectContextOrWait(ctx, 10*time.Millisecond)
			col, err := validRegion.CollidesWith(spatialmath.NewPoint(r3.Vector{X: current[0].Value, Y: current[1].Value}, ""))
			if err != nil {
				movementErr <- err
				return
			}
			if !col {
				movementErr <- errors.New("base has deviated too far from path")
				return
			}

			// get to the x, y location first - note that from the base's perspective +y is forward
			desiredHeading := math.Atan2(desired[1].Value-current[1].Value, desired[0].Value-current[0].Value)
			commanded, err := ddk.issueCommand(cancelContext, current, []referenceframe.Input{desired[0], desired[1], {Value: desiredHeading}})
			if err != nil {
				movementErr <- err
				return
			}

			if !commanded {
				// no command to move to the x, y location was issued, correct the heading and then exit
				// 2DOF model indicates position-only mode so heading doesn't need to be corrected, exit function
				if len(ddk.planningFrame.DoF()) == 2 {
					movementErr <- err
					return
				}
				if commanded, err := ddk.issueCommand(cancelContext, current, []referenceframe.Input{current[0], current[1], desired[2]}); err == nil {
					if !commanded {
						movementErr <- nil
						return
					}
				} else {
					movementErr <- err
					return
				}
			}
			current, err = ddk.CurrentInputs(cancelContext)
			if err != nil {
				movementErr <- err
				return
			}
			ddk.logger.Infof("current inputs: %v", current)
		}
		movementErr <- err
	})

	// watching for movement timeout
	lastUpdate := time.Now()
	var prevInputs []referenceframe.Input

	for {
		utils.SelectContextOrWait(ctx, 100*time.Millisecond)
		select {
		case err := <-movementErr:
			return err
		default:
		}

		currentInputs, err := ddk.CurrentInputs(ctx)
		if err != nil {
			cancel()
			<-movementErr
			return err
		}
		if prevInputs == nil {
			prevInputs = currentInputs
		}
		positionChange := ik.L2InputMetric(&ik.Segment{
			StartConfiguration: prevInputs,
			EndConfiguration:   currentInputs,
		})
		if positionChange > ddk.options.MinimumMovementThresholdMM {
			lastUpdate = time.Now()
			prevInputs = currentInputs
		} else if time.Since(lastUpdate) > ddk.options.Timeout {
			cancel()
			<-movementErr
			return ErrMovementTimeout
		}
	}
}

// issueCommand issues a relevant command to move the base to the given desired inputs and returns the boolean describing
// if it issued a command successfully.  If it is already at the location it will not need to issue another command and can therefore
// return a false.
func (ddk *differentialDriveKinematics) issueCommand(ctx context.Context, current, desired []referenceframe.Input) (bool, error) {
	distErr, headingErr, err := ddk.inputDiff(current, desired)
	if err != nil {
		return false, err
	}
	ddk.logger.Debugf("distErr: %.2f\theadingErr %.2f", distErr, headingErr)
	if distErr > ddk.options.GoalRadiusMM && math.Abs(headingErr) > ddk.options.HeadingThresholdDegrees {
		// base is headed off course; spin to correct
		err := ddk.Spin(ctx, math.Min(headingErr, ddk.options.MaxSpinAngleDeg), ddk.options.AngularVelocityDegsPerSec, nil)

		// Update the cached current inputs to the resultant position of the spin command when the localizer is nil
		if ddk.Localizer == nil {
			ddk.mutex.Lock()
			defer ddk.mutex.Unlock()
			ddk.noLocalizerCacheInputs = []referenceframe.Input{{Value: 0}, {Value: 0}, desired[2]}
			time.Sleep(defaultNoLocalizerDelay)
		}
		return true, err
	} else if distErr > ddk.options.GoalRadiusMM {
		// base is pointed the correct direction but not there yet; forge onward
		err := ddk.MoveStraight(ctx, int(math.Min(distErr, ddk.options.MaxMoveStraightMM)), ddk.options.LinearVelocityMMPerSec, nil)

		// Update the cached current inputs to the resultant position of the move straight command when the localizer is nil
		if ddk.Localizer == nil {
			ddk.mutex.Lock()
			defer ddk.mutex.Unlock()
			ddk.noLocalizerCacheInputs = desired
			time.Sleep(defaultNoLocalizerDelay)
		}
		return true, err
	}
	return false, nil
}

// create a function for the error state, which is defined as [positional error, heading error].
func (ddk *differentialDriveKinematics) inputDiff(current, desired []referenceframe.Input) (float64, float64, error) {
	// create a goal pose in the world frame
	goal := spatialmath.NewPose(
		r3.Vector{X: desired[0].Value, Y: desired[1].Value},
		&spatialmath.OrientationVector{OZ: 1, Theta: desired[2].Value},
	)

	// transform the goal pose such that it is in the base frame
	currentPose, err := ddk.executionFrame.Transform(current)
	if err != nil {
		return 0, 0, err
	}
	delta := spatialmath.PoseBetween(currentPose, goal)

	// calculate the error state
	headingErr := math.Mod(delta.Orientation().OrientationVectorDegrees().Theta, 360)
	positionErr := delta.Point().Norm()
	return positionErr, headingErr, nil
}

// newValidRegionCapsule returns a capsule which defines the valid regions for a base to be when moving to a waypoint.
// The valid region is all points that are deviationThreshold (mm) distance away from the line segment between the
// starting and ending waypoints. This capsule is used to detect whether a base leaves this region and has thus deviated
// too far from its path.
func (ddk *differentialDriveKinematics) newValidRegionCapsule(starting, desired []referenceframe.Input) (spatialmath.Geometry, error) {
	pt := r3.Vector{X: (desired[0].Value + starting[0].Value) / 2, Y: (desired[1].Value + starting[1].Value) / 2}
	positionErr, _, err := ddk.inputDiff(starting, []referenceframe.Input{desired[0], desired[1], {Value: 0}})
	if err != nil {
		return nil, err
	}

	desiredHeading := math.Atan2(starting[0].Value-desired[0].Value, starting[1].Value-desired[1].Value)

	// rotate such that y is forward direction to match the frame for movement of a base
	// rotate around the z-axis such that the capsule points in the direction of the end waypoint
	r, err := spatialmath.NewRotationMatrix([]float64{
		math.Cos(desiredHeading), -math.Sin(desiredHeading), 0,
		0, 0, -1,
		math.Sin(desiredHeading), math.Cos(desiredHeading), 0,
	})
	if err != nil {
		return nil, err
	}

	center := spatialmath.NewPose(pt, r)
	capsule, err := spatialmath.NewCapsule(
		center,
		ddk.options.PlanDeviationThresholdMM,
		2*ddk.options.PlanDeviationThresholdMM+positionErr,
		"")
	if err != nil {
		return nil, err
	}

	return capsule, nil
}

func (ddk *differentialDriveKinematics) ErrorState(
	ctx context.Context,
	plan [][]referenceframe.Input,
	currentNode int,
) (spatialmath.Pose, error) {
	if currentNode <= 0 || currentNode >= len(plan) {
		return nil, fmt.Errorf("cannot get ErrorState for node %d, must be > 0 and less than plan length %d", currentNode, len(plan))
	}

	// Get pose-in-frame of the base via its localizer. The offset between the localizer and its base should already be accounted for.
	actualPIF, err := ddk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}

	var nominalPose spatialmath.Pose

	// Determine the nominal pose, that is, the pose where the robot ought be if it had followed the plan perfectly up until this point.
	// This is done differently depending on what sort of frame we are working with.
	if len(plan) < 2 {
		return nil, errors.New("diff drive motion plan must have at least two waypoints")
	}
	nominalPose, err = ddk.planningFrame.Transform(plan[currentNode])
	if err != nil {
		return nil, err
	}
	pastPose, err := ddk.planningFrame.Transform(plan[currentNode-1])
	if err != nil {
		return nil, err
	}
	// diff drive bases don't have a notion of "distance along the trajectory between waypoints", so instead we compare to the
	// nearest point on the straight line path.
	nominalPoint := spatialmath.ClosestPointSegmentPoint(pastPose.Point(), nominalPose.Point(), actualPIF.Pose().Point())
	pointDiff := nominalPose.Point().Sub(pastPose.Point())
	desiredHeading := math.Atan2(pointDiff.Y, pointDiff.X)
	nominalPose = spatialmath.NewPose(nominalPoint, &spatialmath.OrientationVector{OZ: 1, Theta: desiredHeading})

	return spatialmath.PoseBetween(nominalPose, actualPIF.Pose()), nil
}

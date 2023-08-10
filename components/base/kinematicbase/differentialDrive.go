// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// ErrMovementTimeout is used for when a movement call times out after no movement for some time.
var ErrMovementTimeout = errors.New("movement has timed out")

// wrapWithDifferentialDriveKinematics takes a wheeledBase component and adds a localizer to it
// It also adds kinematic model so that it can be controlled.
func wrapWithDifferentialDriveKinematics(
	ctx context.Context,
	b base.Base,
	logger golog.Logger,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	options Options,
) (KinematicBase, error) {
	ddk := &differentialDriveKinematics{
		Base:      b,
		logger:    logger,
		localizer: localizer,
		options:   options,
	}

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
	ddk.executionFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}

	if options.PositionOnlyMode {
		ddk.planningFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits[:2], geometry)
		if err != nil {
			return nil, err
		}
	} else {
		ddk.planningFrame = ddk.executionFrame
	}

	ddk.fs = referenceframe.NewEmptyFrameSystem("")
	if err := ddk.fs.AddFrame(ddk.executionFrame, ddk.fs.World()); err != nil {
		return nil, err
	}
	return ddk, nil
}

type differentialDriveKinematics struct {
	base.Base
	logger                        golog.Logger
	localizer                     motion.Localizer
	planningFrame, executionFrame referenceframe.Model
	fs                            referenceframe.FrameSystem
	options                       Options
}

func (ddk *differentialDriveKinematics) Kinematics() referenceframe.Frame {
	return ddk.planningFrame
}

func (ddk *differentialDriveKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// TODO(rb): make a transformation from the component reference to the base frame
	pif, err := ddk.localizer.CurrentPosition(ctx)
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
			commanded, err := ddk.issueCommand(cancelContext, current, []referenceframe.Input{desired[0], desired[1], {desiredHeading}})
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
		positionChange := motionplan.L2InputMetric(&motionplan.Segment{
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
	distErr, headingErr, err := ddk.errorState(current, desired)
	if err != nil {
		return false, err
	}
	ddk.logger.Debug("distErr: %f\theadingErr %f", distErr, headingErr)
	if distErr > ddk.options.GoalRadiusMM && math.Abs(headingErr) > ddk.options.HeadingThresholdDegrees {
		// base is headed off course; spin to correct
		return true, ddk.Spin(ctx, math.Min(headingErr, ddk.options.MaxSpinAngleDeg), ddk.options.AngularVelocityDegsPerSec, nil)
	} else if distErr > ddk.options.GoalRadiusMM {
		// base is pointed the correct direction but not there yet; forge onward
		return true, ddk.MoveStraight(ctx, int(distErr), ddk.options.LinearVelocityMMPerSec, nil)
	}
	return false, nil
}

// create a function for the error state, which is defined as [positional error, heading error].
func (ddk *differentialDriveKinematics) errorState(current, desired []referenceframe.Input) (float64, float64, error) {
	// create a goal pose in the world frame
	goal := referenceframe.NewPoseInFrame(
		referenceframe.World,
		spatialmath.NewPose(
			r3.Vector{X: desired[0].Value, Y: desired[1].Value},
			&spatialmath.OrientationVector{OZ: 1, Theta: desired[2].Value},
		),
	)

	// transform the goal pose such that it is in the base frame
	tf, err := ddk.fs.Transform(map[string][]referenceframe.Input{ddk.planningFrame.Name(): current}, goal, ddk.planningFrame.Name())
	if err != nil {
		return 0, 0, err
	}
	delta, ok := tf.(*referenceframe.PoseInFrame)
	if !ok {
		return 0, 0, errors.New("can't interpret transformable as a pose in frame")
	}

	// calculate the error state
	headingErr := math.Mod(delta.Pose().Orientation().OrientationVectorDegrees().Theta, 360)
	positionErr := delta.Pose().Point().Norm()
	return positionErr, headingErr, nil
}

// CollisionGeometry returns a spherical geometry that will encompass the base if it were to rotate the geometry specified in the config
// 360 degrees about the Z axis of the reference frame specified in the config.
func CollisionGeometry(cfg *referenceframe.LinkConfig) ([]spatialmath.Geometry, error) {
	// TODO(RSDK-1014): the orientation of this model will matter for collision checking,
	// and should match the convention of +Y being forward for bases
	if cfg == nil || cfg.Geometry == nil {
		return nil, errors.New("not configured with a geometry use caution if using motion service - collisions will not be accounted for")
	}
	geoCfg := cfg.Geometry
	r := geoCfg.TranslationOffset.Norm()
	switch geoCfg.Type {
	case spatialmath.BoxType:
		r += r3.Vector{X: geoCfg.X, Y: geoCfg.Y, Z: geoCfg.Z}.Norm() / 2
	case spatialmath.SphereType:
		r += geoCfg.R
	case spatialmath.CapsuleType:
		r += geoCfg.L / 2
	case spatialmath.UnknownType:
		// no type specified, iterate through supported types and try to infer intent
		if norm := (r3.Vector{X: geoCfg.X, Y: geoCfg.Y, Z: geoCfg.Z}).Norm(); norm > 0 {
			r += norm / 2
		} else if geoCfg.L != 0 {
			r += geoCfg.L / 2
		} else {
			r += geoCfg.R
		}
	case spatialmath.PointType:
	default:
		return nil, spatialmath.ErrGeometryTypeUnsupported
	}
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), r, geoCfg.Label)
	if err != nil {
		return nil, err
	}
	return []spatialmath.Geometry{sphere}, nil
}

// newValidRegionCapsule returns a capsule which defines the valid regions for a base to be when moving to a waypoint.
// The valid region is all points that are deviationThreshold (mm) distance away from the line segment between the
// starting and ending waypoints. This capsule is used to detect whether a base leaves this region and has thus deviated
// too far from its path.
func (ddk *differentialDriveKinematics) newValidRegionCapsule(starting, desired []referenceframe.Input) (spatialmath.Geometry, error) {
	pt := r3.Vector{X: (desired[0].Value + starting[0].Value) / 2, Y: (desired[1].Value + starting[1].Value) / 2}
	positionErr, _, err := ddk.errorState(starting, []referenceframe.Input{desired[0], desired[1], {0}})
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

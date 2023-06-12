package wheeled

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

const (
	distThresholdMM         = 100
	headingThresholdDegrees = 30
	defaultAngularVelocity  = 60  // degrees per second
	defaultLinearVelocity   = 300 // mm per second
)

type kinematicWheeledBase struct {
	*wheeledBase
	localizer motion.Localizer
	model     referenceframe.Model
	fs        referenceframe.FrameSystem
}

// WrapWithKinematics takes a wheeledBase component and adds a slam service to it
// It also adds kinematic model so that it can be controlled.
func (wb *wheeledBase) WrapWithKinematics(
	ctx context.Context,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
) (base.KinematicBase, error) {
	geometry, err := base.CollisionGeometry(wb.frame)
	if err != nil {
		return nil, err
	}
	model, err := referenceframe.New2DMobileModelFrame(wb.name, limits, geometry)
	if err != nil {
		return nil, err
	}
	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(model, fs.World()); err != nil {
		return nil, err
	}
	return &kinematicWheeledBase{
		wheeledBase: wb,
		localizer:   localizer,
		model:       model,
		fs:          fs,
	}, nil
}

func (kwb *kinematicWheeledBase) ModelFrame() referenceframe.Model {
	return kwb.model
}

func (kwb *kinematicWheeledBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// TODO(rb): make a transformation from the component reference to the base frame
	pif, err := kwb.localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := pif.Pose().Point()
	theta := math.Mod(pif.Pose().Orientation().OrientationVectorRadians().Theta, 2*math.Pi) - math.Pi
	return []referenceframe.Input{{Value: pt.X}, {Value: pt.Y}, {Value: theta}}, nil
}

func (kwb *kinematicWheeledBase) GoToInputs(ctx context.Context, desired []referenceframe.Input) (err error) {
	// this loop polls the error state and issues a corresponding command to move the base to the objective
	// when the base is within the positional threshold of the goal, exit the loop
	for err = ctx.Err(); err == nil; err = ctx.Err() {
		current, err := kwb.CurrentInputs(ctx)
		kwb.logger.Warnf("current inputs: %v", current)
		kwb.logger.Warnf("desired inputs: %v", desired)
		if err != nil {
			return err
		}

		// get to the x, y location first - note that from the base's perspective +y is forward
		desiredHeading := math.Atan2(current[1].Value-desired[1].Value, current[0].Value-desired[0].Value)
		if commanded, err := kwb.issueCommand(ctx, current, []referenceframe.Input{desired[0], desired[1], {desiredHeading}}); err == nil {
			if commanded {
				continue
			}
		}

		// no command to move to the x, y location was issued, correct the heading and then exit
		if commanded, err := kwb.issueCommand(ctx, current, []referenceframe.Input{current[0], current[1], desired[2]}); err == nil {
			if !commanded {
				return nil
			}
		}
		time.Sleep(time.Millisecond*500)
	}
	return err
}

// issueCommand issues a relevant command to move the base to the given desired inputs and returns the boolean describing
// if it issued a command successfully.  If it is already at the location it will not need to issue another command and can therefore
// return a false.
func (kwb *kinematicWheeledBase) issueCommand(ctx context.Context, current, desired []referenceframe.Input) (bool, error) {
	distErr, headingErr, err := kwb.errorState(current, desired)
	if err != nil {
		return false, err
	}
	if distErr > distThresholdMM && math.Abs(headingErr) > headingThresholdDegrees {
		// base is headed off course; spin to correct
		kwb.logger.Warnf("spinning to course correct %v degrees", headingErr)
		return true, kwb.Spin(ctx, -headingErr, defaultAngularVelocity, nil)
	} else if distErr > distThresholdMM {
		// base is pointed the correct direction but not there yet; forge onward
		kwb.logger.Warnf("driving straight")
		return true, kwb.MoveStraight(ctx, distErr, defaultLinearVelocity, nil)
	}
	return false, nil
}

// create a function for the error state, which is defined as [positional error, heading error].
func (kwb *kinematicWheeledBase) errorState(current, desired []referenceframe.Input) (int, float64, error) {
	// create a goal pose in the world frame
	goal := referenceframe.NewPoseInFrame(
		referenceframe.World,
		spatialmath.NewPose(
			r3.Vector{X: desired[0].Value, Y: desired[1].Value},
			&spatialmath.OrientationVector{OZ: 1, Theta: desired[2].Value},
		),
	)

	// transform the goal pose such that it is in the base frame
	tf, err := kwb.fs.Transform(map[string][]referenceframe.Input{kwb.name: current}, goal, kwb.name)
	if err != nil {
		return 0, 0, err
	}
	delta, ok := tf.(*referenceframe.PoseInFrame)
	if !ok {
		return 0, 0, errors.New("can't interpret transformable as a pose in frame")
	}

	// calculate the error state
	headingErr := math.Mod(delta.Pose().Orientation().OrientationVectorDegrees().Theta, 360)
	positionErr := int(delta.Pose().Point().Norm())
	return positionErr, headingErr, nil
}

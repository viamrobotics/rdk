package wheeled

import (
	"context"
	"math"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type kinematicWheeledBase struct {
	*wheeledBase
	slam  slam.Service
	model referenceframe.Model
}

// WrapWithKinematics takes a wheeledBase component and adds a slam service to it
// It also adds kinematic model so that it can be controlled.
func (base *wheeledBase) WrapWithKinematics(ctx context.Context, slamSvc slam.Service) (base.KinematicBase, error) {
	var err error
	wb, ok := utils.UnwrapProxy(base).(*wheeledBase)
	if !ok {
		return nil, errors.Errorf("could not interpret base of type %T as a wheeledBase", base)
	}
	kwb := &kinematicWheeledBase{
		wheeledBase: wb,
		slam:        slamSvc,
	}
	limits, err := slam.Limits(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	kwb.model, err = Model(kwb.name, kwb.collisionGeometry, limits)
	if err != nil {
		return nil, err
	}
	return kwb, err
}

func (kwb *kinematicWheeledBase) ModelFrame() referenceframe.Model {
	return kwb.model
}

func (kwb *kinematicWheeledBase) currentPose(ctx context.Context) (spatialmath.Pose, error) {
	// TODO: make a transformation from the component reference to the base frame
	pose, _, err := kwb.slam.GetPosition(ctx)
	return pose, err
}

func (kwb *kinematicWheeledBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// TODO: make a transformation from the component reference to the base frame
	pose, err := kwb.currentPose(ctx)
	if err != nil {
		return nil, err
	}
	pt := pose.Point()

	// Need to get X, Z from lidar because Y points down
	// TODO: make a ticket to give rplidar kinematic information so that you don't have to do this here
	return []referenceframe.Input{{Value: pt.X}, {Value: pt.Z}}, nil
}

func (kwb *kinematicWheeledBase) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	if kwb.collisionGeometry == nil {
		return errors.New("cannot move base without a collision geometry")
	}

	// TODO: may want to save the startPose separately
	// TODO: code janitor
	currentPose, err := kwb.currentPose(ctx)
	if err != nil {
		return err
	}
	currentPt := currentPose.Point()
	desiredHeading := math.Atan2(currentPt.Z-goal[1].Value, currentPt.X-goal[0].Value)
	distance := math.Hypot(currentPt.Z-goal[1].Value, currentPt.X-goal[0].Value)

	// TODO: we do want the pitch here but this is domain limited to -90 to 90, need math to fix this
	heading := utils.RadToDeg(currentPose.Orientation().EulerAngles().Pitch)
	// While base is not at the goal
	// TO DO figure out sane threshold.
	for distance > 5 {
		// If heading is ok, go forward
		// Otherwise spin until base is heading correct way
		// TODO make a threshold
		if math.Abs(heading-desiredHeading) > 0 {
			// TODO (rh) create context with cancel
			// TODO use a speed that is not garbage
			if err := kwb.Spin(ctx, heading-desiredHeading, 10, nil); err != nil {
				return err
			}
		} else {
			// TODO check if we are in mm in SLAM and multiply by 1000 if so
			distance := math.Hypot(currentPt.Z-goal[1].Value, currentPt.X-goal[0].Value)
			if err := kwb.MoveStraight(ctx, int(distance), 10, nil); err != nil {
				return err
			}

		}

		// Calculate current state
		currentPose, err = kwb.currentPose(ctx)
		if err != nil {
			return err
		}
		currentPt = currentPose.Point()
		heading = utils.RadToDeg(currentPose.Orientation().EulerAngles().Pitch)
		distance = math.Hypot(currentPt.Z-goal[1].Value, currentPt.X-goal[0].Value)

	}
	return nil
}

// ModelFrame builds the kinematic model associated with the kinematicWheeledBase
// Note that this model is not intended to be registered in the frame system.
func Model(name string, collisionGeometry spatialmath.Geometry, limits []referenceframe.Limit) (referenceframe.Model, error) {
	// build the model - SLAM convention is that the XZ plane is the ground plane
	frame2D, err := referenceframe.NewMobile2DFrame(collisionGeometry.Label(), limits, collisionGeometry)
	if err != nil {
		return nil, err
	}
	model := referenceframe.NewSimpleModel(name)
	model.OrdTransforms = []referenceframe.Frame{frame2D}
	return model, nil
}

package wheeled

import (
	"bytes"
	"context"
	"math"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type kinematicWheeledBase struct {
	*wheeledBase
	slam slam.Service
	// TODO(RSDK-2393): When SLAM services know their own name this can be taken out
	slamName string
	model    referenceframe.Model
}

// WrapWithKinematics takes a wheeledBase component and adds a slam service to it
// It also adds kinematic model so that it can be controlled.
func (wb *wheeledBase) WrapWithKinematics(ctx context.Context, slamSvc slam.Service) (base.KinematicBase, error) {
	// gets the extents of the SLAM map
	data, err := slam.GetPointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	dims, err := pointcloud.GetPCDMetaData(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	geometry, err := base.CollisionGeometry(wb.frame)
	if err != nil {
		return nil, err
	}
	model, err := Model(wb.name, geometry, []referenceframe.Limit{{Min: dims.MinX, Max: dims.MaxX}, {Min: dims.MinZ, Max: dims.MaxZ}})
	if err != nil {
		return nil, err
	}
	return &kinematicWheeledBase{
		wheeledBase: wb,
		slam:        slamSvc,
		model:       model,
	}, err
}

func (kwb *kinematicWheeledBase) ModelFrame() referenceframe.Model {
	return kwb.model
}

func (kwb *kinematicWheeledBase) currentPose(ctx context.Context) (spatialmath.Pose, error) {
	// TODO: make a transformation from the component reference to the base frame
	pose, _, err := kwb.slam.GetPosition(ctx, kwb.slamName)
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

// Model builds the kinematic model associated with the kinematicWheeledBase
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

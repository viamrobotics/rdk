package wheeled

import (
	"bytes"
	"context"
	"errors"
	"math"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

const (
	positionThresholdM      = 10
	headingThresholdDegrees = 15
)

type kinematicWheeledBase struct {
	*wheeledBase
	slam  slam.Service
	model referenceframe.Model
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
	model, err := MakeModelFrame(
		wb.name,
		geometry,
		[]referenceframe.Limit{{Min: dims.MinX, Max: dims.MaxX}, {Min: dims.MinY, Max: dims.MaxY}, {Min: -math.Pi, Max: math.Pi}},
	)
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

func (kwb *kinematicWheeledBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// TODO: make a transformation from the component reference to the base frame
	pose, _, err := kwb.slam.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := pose.Point()
	theta := pose.Orientation().OrientationVectorRadians().Theta

	// Need to get X, Z from lidar because Y points down
	// TODO: make a ticket to give rplidar kinematic information so that you don't have to do this here
	return []referenceframe.Input{{Value: pt.X}, {Value: pt.Z}, {Value: theta}}, nil
}

func (kwb *kinematicWheeledBase) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	errorState := func() (int, float64, error) {
		currentInputs, err := kwb.CurrentInputs(ctx)
		if err != nil {
			return 0, 0, err
		}

		fs := referenceframe.NewEmptySimpleFrameSystem("kwb")
		if err := fs.AddFrame(kwb.model, fs.World()); err != nil {
			return 0, 0, err
		}
		goal := referenceframe.NewPoseInFrame(
			referenceframe.World,
			spatialmath.NewPoseFromPoint(r3.Vector{inputs[0].Value, inputs[1].Value, 0}),
		)
		tf, err := fs.Transform(map[string][]referenceframe.Input{kwb.name: currentInputs}, goal, kwb.name)
		if err != nil {
			return 0, 0, err
		}
		delta, ok := tf.(*referenceframe.PoseInFrame)
		if !ok {
			return 0, 0, errors.New("can't interpret transformable as a pose in frame")
		}
		heading := math.Mod(delta.Pose().Orientation().OrientationVectorDegrees().Theta, 360)
		headingErr := math.Min(heading, 360-heading)
		positionErr := int(1000 * delta.Pose().Point().Norm())
		kwb.logger.Warnf("POSITION ERROR: \t%d MM\tHEADING ERROR: \t%f DEGREES", positionErr, headingErr)
		return positionErr, headingErr, nil
	}

	// TODO: we do want the pitch here but this is domain limited to -90 to 90, need math to fix this
	// heading := utils.RadToDeg(currentPose.Orientation().EulerAngles().Pitch)
	// While base is not at the goal
	// TO DO figure out sane threshold.

	positionErr, headingErr, err := errorState()
	if err != nil {
		return err
	}

	for positionErr > positionThresholdM {
		// If heading is ok, go forward
		// Otherwise spin until base is heading correct way
		// TODO make a threshold
		// if math.Abs(headingErr) > headingThresholdDegrees {
		// 	// TODO (rh) create context with cancel
		// 	// TODO use a speed that is not garbage
		// 	if err := kwb.Spin(ctx, -headingErr, 5, nil); err != nil {
		// 		return err
		// 	}
		// } else {
		// 	if err := kwb.MoveStraight(ctx, -positionErr, 300, nil); err != nil {
		// 		return err
		// 	}

		// }

		positionErr, headingErr, err = errorState()
		if err != nil {
			return err
		}
	}

	_ = positionErr
	_ = headingErr
	return nil
}

// MakeModelFrame builds the kinematic model associated with the kinematicWheeledBase
// Note that this model is not intended to be registered in the frame system.
func MakeModelFrame(name string, collisionGeometry spatialmath.Geometry, limits []referenceframe.Limit) (referenceframe.Model, error) {
	// build the model - SLAM convention is that the XY plane is the ground plane
	frame2D, err := referenceframe.NewMobile2DFrame(collisionGeometry.Label(), limits, collisionGeometry)
	if err != nil {
		return nil, err
	}
	model := referenceframe.NewSimpleModel(name)
	model.OrdTransforms = []referenceframe.Frame{frame2D}
	return model, nil
}

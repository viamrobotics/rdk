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
	positionThresholdMM     = 100
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
	limits := []referenceframe.Limit{
		{Min: dims.MinX, Max: dims.MaxX},
		{Min: dims.MinY, Max: dims.MaxY},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}
	model, err := referenceframe.New2DMobileModelFrame(wb.name, limits, geometry)
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
	// TODO(rb): make a transformation from the component reference to the base frame
	pose, _, err := kwb.slam.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := pose.Point()
	theta := math.Mod(pose.Orientation().OrientationVectorRadians().Theta, 2*math.Pi) - math.Pi
	return []referenceframe.Input{{Value: pt.X}, {Value: pt.Y}, {Value: theta}}, nil
}

func (kwb *kinematicWheeledBase) GoToInputs(ctx context.Context, inputs []referenceframe.Input) (err error) {
	// create a frame system to be used during the context of this function, used to locate the goal in the base frame
	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(kwb.model, fs.World()); err != nil {
		return err
	}

	// create a function for the error state, which is defined as [positional error, heading error]
	errorState := func() (int, float64, error) {
		currentInputs, err := kwb.CurrentInputs(ctx)
		if err != nil {
			return 0, 0, err
		}

		// create a goal pose in the world frame
		desiredHeading := math.Atan2(currentInputs[1].Value-inputs[1].Value, currentInputs[0].Value-inputs[0].Value)
		goal := referenceframe.NewPoseInFrame(
			referenceframe.World,
			spatialmath.NewPose(
				r3.Vector{X: inputs[0].Value, Y: inputs[1].Value},
				&spatialmath.OrientationVector{OZ: 1, Theta: desiredHeading},
			),
		)

		// transform the goal pose such that it is in the base frame
		tf, err := fs.Transform(map[string][]referenceframe.Input{kwb.name: currentInputs}, goal, kwb.name)
		if err != nil {
			return 0, 0, err
		}
		delta, ok := tf.(*referenceframe.PoseInFrame)
		if !ok {
			return 0, 0, errors.New("can't interpret transformable as a pose in frame")
		}

		// calculate the error state
		headingErr := math.Mod(delta.Pose().Orientation().OrientationVectorDegrees().Theta, 360)
		positionErr := int(1000 * delta.Pose().Point().Norm())
		return positionErr, headingErr, nil
	}

	// this loop polls the error state and issues a corresponding command to move the base to the objective
	// when the base is within the positional threshold of the goal, exit the loop
	// TODO(rb): check for the context being cancelled and stop if so
	for distErr, headingErr, err := errorState(); err == nil && distErr > positionThresholdMM; distErr, headingErr, err = errorState() {
		if math.Abs(headingErr) > headingThresholdDegrees {
			err = kwb.Spin(ctx, -headingErr, 60, nil) // base is headed off course; spin to correct
		} else {
			err = kwb.MoveStraight(ctx, distErr, 300, nil) // base is pointed the correct direction; forge onward
		}
		if err != nil {
			return err
		}
	}

	return err
}

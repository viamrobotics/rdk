//go:build !no_cgo

// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var errArmClientModelNotValid = errors.New("unable to retrieve a valid arm model from arm client")

// client implements ArmServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.ArmServiceClient
	model  referenceframe.Model
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Arm, error) {
	pbClient := pb.NewArmServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: pbClient,
		logger: logger,
	}
	clientFrame, err := c.updateKinematics(ctx, nil)
	if err != nil {
		logger.CErrorw(ctx, "error getting model for arm; will not allow certain methods", "err", err)
	} else {
		c.model = clientFrame
	}
	return c, nil
}

// EndPosition gets the current position of the arm as a Pose.
//
//    myArm, err := arm.FromRobot(robot, "my_arm")
//    // Get the end position of the arm as a Pose.
//    err, pos := myArm.EndPosition(context.Background(), nil)
func (c *client) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetEndPosition(ctx, &pb.GetEndPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPoseFromProtobuf(resp.Pose), nil
}

// MoveToPosition moves the end of the arm to the desired Pose, relative to the base of the arm.
//
//    myArm, err := arm.FromRobot(robot, "my_arm")
//    // Create a Pose for the arm.
//    examplePose = []float64{x: 5, y: 5, z: 5, o_x: 5, o_y: 5, o_z: 5, theta:20}
//
//    // Move your arm to the Pose.
//    err := myArm.MoveToPosition(context.Background(), pose: examplePose, nil)
func (c *client) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:  c.name,
		To:    spatialmath.PoseToProtobuf(pose),
		Extra: ext,
	})
	return err
}

// MoveToJointPositions moves the end of the arm to the desired Pose, relative to the base of the arm.
//
//    myArm, err := arm.FromRobot(robot, "my_arm")
//
//    // Declare an array of values with your desired rotational value for each joint on the arm.
//    degrees := []float64{4.0, 5.0, 6.0}
//
//    // Declare a new JointPositions with these values.
//    jointPos := componentpb.JointPositions{degrees}
//
//    // Move each joint of the arm to the position these values specify.
//    err := myArm.MoveToJointPositions(context.Background(), jointPos, nil)
func (c *client) MoveToJointPositions(ctx context.Context, positions *pb.JointPositions, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToJointPositions(ctx, &pb.MoveToJointPositionsRequest{
		Name:      c.name,
		Positions: positions,
		Extra:     ext,
	})
	return err
}

// JointPositions gets the current position of each joint on the arm.
// 
//    my_arm, err := arm.FromRobot(robot, "my_arm")
//
//    // Get the current position of each joint on the arm as JointPositions.
//    pos, err := my_arm.JointPositions(context.Background(), nil)
func (c *client) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetJointPositions(ctx, &pb.GetJointPositionsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

// Stop will stop all motion of the arm.
//
//    myArm, err := arm.FromRobot(robot, "my_arm")
//    // Stop all motion of the arm. It is assumed that the arm stops immediately.
//    err := myArm.Stop(context.Background(), nil)
func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	return c.model
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if c.model == nil {
		return nil, errArmClientModelNotValid
	}
	resp, err := c.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return c.model.InputFromProtobuf(resp), nil
}

func (c *client) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	if c.model == nil {
		return errArmClientModelNotValid
	}
	for _, goal := range inputSteps {
		err := c.MoveToJointPositions(ctx, c.model.ProtobufFromInput(goal), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// DoCommand executes model-specific commands that are not otherwise defined by the component API.
//
//    myArm, err := arm.FromRobot(robot, "my_arm")
//
//    command := map[string]interface{}{"cmd": "test", "data1": 500}
//    result, err := myArm.DoCommand(context.Background(), command)
func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

// IsMoving gets if the arm is currently moving.
//
//    myArm, err := arm.FromRobot(robot, "my_arm")
//
//    // Stop all motion of the arm. It is assumed that the arm stops immediately.
//    myArm.Stop(context.Background(), nil)
//
//    // Log if the arm is currently moving.
//    is_moving, err := myArm.IsMoving(context.Background())
//    logger.Info(is_moving)
func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

// Geometries gets all current geometries of the arm, positioned relative to the arm's frame.
//    myArm, err := arm.FromRobot(robot, "my_arm")
//
//    geometries, err := myArm.Geometries(context.Background(), nil)
//
//    if len(geometries) > 0 {
//     	  // Get the center of the first geometry
//        elem := geometries[0]
//        fmt.Println("Pose of the first geometry's center point:", elem.center)
//    }
func (c *client) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetGeometries(ctx, &commonpb.GetGeometriesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewGeometriesFromProto(resp.GetGeometries())
}

func (c *client) updateKinematics(ctx context.Context, extra map[string]interface{}) (referenceframe.Model, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetKinematics(ctx, &commonpb.GetKinematicsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}

	format := resp.GetFormat()
	data := resp.GetKinematicsData()

	switch format {
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA:
		return referenceframe.UnmarshalModelJSON(data, c.name)
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF:
		modelconf, err := urdf.UnmarshalModelXML(data, c.name)
		if err != nil {
			return nil, err
		}
		return modelconf.ParseConfig(c.name)
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED:
		fallthrough
	default:
		if formatName, ok := commonpb.KinematicsFileFormat_name[int32(format)]; ok {
			return nil, fmt.Errorf("unable to parse file of type %s", formatName)
		}
		return nil, fmt.Errorf("unable to parse unknown file type %d", format)
	}
}
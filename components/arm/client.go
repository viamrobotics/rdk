//go:build !no_cgo

// Package arm contains a gRPC based arm client.
package arm

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

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
		logger.CWarnw(
			ctx,
			"error getting model for arm; making the assumption that joints are revolute and that their positions are specified in degrees",
			"err",
			err,
		)
	} else {
		c.model = clientFrame
	}
	return c, nil
}

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

func (c *client) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if pose == nil {
		c.logger.Warnf("%s MoveToPosition: pose parameter is nil", c.name)
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:  c.name,
		To:    spatialmath.PoseToProtobuf(pose),
		Extra: ext,
	})
	return err
}

func (c *client) MoveToJointPositions(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if positions == nil {
		c.logger.Warnf("%s MoveToJointPositions: position parameter is nil", c.name)
	}
	jp, err := referenceframe.JointPositionsFromInputs(c.model, positions)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToJointPositions(ctx, &pb.MoveToJointPositionsRequest{
		Name:      c.name,
		Positions: jp,
		Extra:     ext,
	})
	return err
}

func (c *client) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
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
	return referenceframe.InputsFromJointPositions(c.model, resp.Positions)
}

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
	return c.JointPositions(ctx, nil)
}

func (c *client) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	// TODO: switch this over call MoveThroughJointPositions when that API is ready
	for _, goal := range inputSteps {
		if err := c.MoveToJointPositions(ctx, goal, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

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

	return parseKinematicsResponse(c.name, resp)
}

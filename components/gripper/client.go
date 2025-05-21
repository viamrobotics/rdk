// Package gripper contains a gRPC based gripper client.
package gripper

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// client implements GripperServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.GripperServiceClient
	logger logging.Logger
	model  referenceframe.Model
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Gripper, error) {
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: pb.NewGripperServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *client) Open(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Open(ctx, &pb.OpenRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	resp, err := c.client.Grab(ctx, &pb.GrabRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
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

func (c *client) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	// if a model has been cached just return that
	if c.model != nil {
		return c.model, nil
	}
	// attempt to get kinematics the correct way
	resp, err := c.client.GetKinematics(ctx, &commonpb.GetKinematicsRequest{Name: c.name})
	if err == nil {
		model, err := referenceframe.KinematicModelFromProtobuf(c.name, resp)
		if err != nil {
			return nil, err
		}
		c.model = model
		return c.model, nil
	}
	// fall back on the old method of providing a model
	geometries, err := c.Geometries(ctx, nil)
	if err == nil {
		return MakeModel(c.name, geometries)
	}
	// if all else fails, we don't want this to error, return a simple model
	return referenceframe.NewSimpleModel(c.name), nil
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	c.logger.Warn("gripper.CurrentInputs is unimplemented!")
	return []referenceframe.Input{}, nil
}

func (c *client) GoToInputs(context.Context, ...[]referenceframe.Input) error {
	c.logger.Warn("gripper.GoToInputs is unimplemented!")
	return nil
}

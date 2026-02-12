// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"sync"

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
	logger logging.Logger

	mu    sync.Mutex
	model referenceframe.Model
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
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: pbClient,
		logger: logger,
	}, nil
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
	m, err := c.Kinematics(ctx)
	if err != nil {
		warnKinematicsUnsafe(ctx, c.logger, err)
	} else if err := CheckDesiredJointPositions(ctx, c, positions); err != nil {
		return err
	}

	jp, err := referenceframe.JointPositionsFromInputs(m, positions)
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

func (c *client) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	options *MoveOptions,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if positions == nil {
		c.logger.Warnf("%s MoveThroughJointPositions: position argument is nil", c.name)
	}
	allJPs := make([]*pb.JointPositions, 0, len(positions))
	hasKinematics := true
	m, err := c.Kinematics(ctx)
	if err != nil {
		hasKinematics = false
		warnKinematicsUnsafe(ctx, c.logger, err)
	}
	for _, position := range positions {
		if hasKinematics {
			if err := CheckDesiredJointPositions(ctx, c, position); err != nil {
				return err
			}
		}
		jp, err := referenceframe.JointPositionsFromInputs(m, position)
		if err != nil {
			return err
		}
		allJPs = append(allJPs, jp)
	}
	req := &pb.MoveThroughJointPositionsRequest{
		Name:      c.name,
		Positions: allJPs,
		Extra:     ext,
	}
	if options != nil {
		req.Options = options.toProtobuf()
	}
	_, err = c.client.MoveThroughJointPositions(ctx, req)
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
	m, err := c.Kinematics(ctx)
	if err != nil {
		warnKinematicsUnsafe(ctx, c.logger, err)
	}
	return referenceframe.InputsFromJointPositions(m, resp.Positions)
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

func (c *client) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// for performance we cache the model after building it once, and can quickly return if its already been created.
	if c.model == nil {
		resp, err := c.client.GetKinematics(ctx, &commonpb.GetKinematicsRequest{Name: c.name})
		if err != nil {
			return nil, err
		}
		model, err := referenceframe.KinematicModelFromProtobuf(c.name, resp)
		if err != nil {
			return nil, err
		}
		c.model = model
	}
	return c.model, nil
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return c.JointPositions(ctx, nil)
}

func (c *client) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	return c.MoveThroughJointPositions(ctx, inputSteps, nil, nil)
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
	return referenceframe.NewGeometriesFromProto(resp.GetGeometries())
}

func (c *client) Get3DModels(ctx context.Context, extra map[string]interface{}) (map[string]*commonpb.Mesh, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Get3DModels(ctx, &commonpb.Get3DModelsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// warnKinematicsUnsafe is a helper function to warn the user that no kinematics have been supplied for the conversion between
// joints space and Inputs. The assumption we are making here is safe for any arm that has only revolute joints (true for most
// commercially available arms) and will only come into play if the kinematics for the arm have not been cached successfully yet.
// The other assumption being made here is that it will be annoying for new users implementing an arm module to not be able to move their
// arm until the kinematics have been supplied.  This log message will be very noisy as it will be logged whenever kinematics are not found
// so we are hoping that they will want to do things the correct way and supply kinematics to quiet it.
func warnKinematicsUnsafe(ctx context.Context, logger logging.Logger, err error) {
	logger.CWarnw(
		ctx,
		"error getting model for arm; making the assumption that joints are revolute and that their positions are specified in degrees",
		"err",
		err,
	)
}

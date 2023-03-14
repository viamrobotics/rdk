// Package motion contains a gRPC based motion client
package motion

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// client implements MotionServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.MotionServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewMotionServiceClient(conn)
	c := &client{
		name:   name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c
}

func (c *client) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	worldStateMsg, err := referenceframe.WorldStateToProtobuf(worldState)
	if err != nil {
		return false, err
	}
	resp, err := c.client.Move(ctx, &pb.MoveRequest{
		Name:            c.name,
		ComponentName:   protoutils.ResourceNameToProto(componentName),
		Destination:     referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:      worldStateMsg,
		SlamServiceName: protoutils.ResourceNameToProto(slamName),
		Extra:           ext,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) (bool, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	worldStateMsg, err := referenceframe.WorldStateToProtobuf(worldState)
	if err != nil {
		return false, err
	}
	resp, err := c.client.MoveSingleComponent(ctx, &pb.MoveSingleComponentRequest{
		Name:          c.name,
		ComponentName: protoutils.ResourceNameToProto(componentName),
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:    worldStateMsg,
		Extra:         ext,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	transforms, err := referenceframe.LinkInFramesToTransformsProtobuf(supplementalTransforms)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetPose(ctx, &pb.GetPoseRequest{
		Name:                   c.name,
		ComponentName:          protoutils.ResourceNameToProto(componentName),
		DestinationFrame:       destinationFrame,
		SupplementalTransforms: transforms,
		Extra:                  ext,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

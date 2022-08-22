// Package motion contains a gRPC based motion client
package motion

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// client implements MotionServiceClient.
type client struct {
	conn   rpc.ClientConn
	client pb.MotionServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewMotionServiceClient(conn)
	c := &client{
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
	worldState *commonpb.WorldState,
) (bool, error) {
	resp, err := c.client.Move(ctx, &pb.MoveRequest{
		ComponentName: protoutils.ResourceNameToProto(componentName),
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:    worldState,
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
	worldState *commonpb.WorldState,
) (bool, error) {
	resp, err := c.client.MoveSingleComponent(ctx, &pb.MoveSingleComponentRequest{
		ComponentName: protoutils.ResourceNameToProto(componentName),
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:    worldState,
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
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	resp, err := c.client.GetPose(ctx, &pb.GetPoseRequest{
		ComponentName:          protoutils.ResourceNameToProto(componentName),
		DestinationFrame:       destinationFrame,
		SupplementalTransforms: supplementalTransforms,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

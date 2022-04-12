// Package motion contains a gRPC based motion client
package motion

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// client is a client satisfies the motion.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.MotionServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewMotionServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.conn.Close()
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Service, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
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

func (c *client) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	resp, err := c.client.GetPose(ctx, &pb.GetPoseRequest{
		ComponentName:    protoutils.ResourceNameToProto(componentName),
		DestinationFrame: destinationFrame,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

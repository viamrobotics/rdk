// Package motion contains a gRPC based motion client
package motion

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/referenceframe"
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
	componentName string,
	destination *referenceframe.PoseInFrame,
	obstacles []*referenceframe.GeometriesInFrame,
) (bool, error) {
	geometriesInFrames := make([]*commonpb.GeometriesInFrame, len(obstacles))
	for i, obstacle := range obstacles {
		geometriesInFrames[i] = referenceframe.GeometriesInFrameToProtobuf(obstacle)
	}
	resp, err := c.client.Move(ctx, &pb.MoveRequest{
		ComponentName: componentName,
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState: &commonpb.WorldState{
			Obstacles: geometriesInFrames,
		},
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

// Package framesystem contains a gRPC based frame system client
package framesystem

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/framesystem/v1"
	"go.viam.com/rdk/referenceframe"
)

// client is a client satisfies the frame_system.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.FrameSystemServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewFrameSystemServiceClient(conn)
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

func (c *client) Config(ctx context.Context, additionalTransforms []*commonpb.Transform) (Parts, error) {
	resp, err := c.client.Config(ctx, &pb.ConfigRequest{
		SupplementalTransforms: additionalTransforms,
	})
	if err != nil {
		return nil, err
	}
	cfgs := resp.GetFrameSystemConfigs()
	result := make([]*config.FrameSystemPart, 0, len(cfgs))
	for _, cfg := range cfgs {
		part, err := config.ProtobufToFrameSystemPart(cfg)
		if err != nil {
			return nil, err
		}
		result = append(result, part)
	}
	return Parts(result), nil
}

func (c *client) TransformPose(
	ctx context.Context,
	query *referenceframe.PoseInFrame,
	destination string,
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	resp, err := c.client.TransformPose(ctx, &pb.TransformPoseRequest{
		Destination:            destination,
		Source:                 referenceframe.PoseInFrameToProtobuf(query),
		SupplementalTransforms: additionalTransforms,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

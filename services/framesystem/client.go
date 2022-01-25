// Package framesystem contains a gRPC based frame system client
package framesystem

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/referenceframe"
)

// client is a client satisfies the framesystem.proto contract.
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

func (c *client) Config(ctx context.Context) ([]*config.FrameSystemPart, error) {
	resp, err := c.client.Config(ctx, &pb.FrameSystemServiceConfigRequest{})
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	cfgs := resp.FrameSystemConfigs
	result := make([]*config.FrameSystemPart, 0, len(cfgs))
	for i, cfg := range cfgs {
		part, err := config.ProtobufToFrameSystemPart(cfg)
		if err != nil {
			return nil, err
		}
		result = append(result, part)
	}
	return result, nil
}

// FrameSystem retrieves an ordered slice of the frame configs and then builds a FrameSystem from the configs.
func (c *client) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	// request the full config from the remote robot's frame system service.FrameSystemConfig()
	parts, err := c.Config(ctx)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		// rename everything with prefixes
		part.Name = prefix + part.Name
		if part.FrameConfig.Parent != referenceframe.World {
			part.FrameConfig.Parent = prefix + part.FrameConfig.Parent
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part, c.logger)
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.GetFrame(part.FrameConfig.Parent))
		if err != nil {
			return nil, err
		}
		err = fs.AddFrame(modelFrame, staticOffsetFrame)
		if err != nil {
			return nil, err
		}
	}
	return fs, nil
}

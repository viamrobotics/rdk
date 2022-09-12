// Package generic contains a gRPC based generic client.
package generic

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	pb "go.viam.com/rdk/proto/api/component/generic/v1"
	"go.viam.com/rdk/protoutils"
)

// client implements GenericServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.GenericServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Generic {
	c := pb.NewGenericServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return DoFromConnection(ctx, c.conn, c.name, cmd)
}

// DoFromConnection is a helper to allow Do() calls from other component clients.
func DoFromConnection(ctx context.Context, conn rpc.ClientConn, name string, cmd map[string]interface{}) (map[string]interface{}, error) {
	gclient := pb.NewGenericServiceClient(conn)
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}

	resp, err := gclient.DoCommand(ctx, &pb.DoCommandRequest{
		Name:    name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}

// Package generic contains a gRPC based generic client.
package generic

import (
	"context"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
)

// client implements GenericServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client genericpb.GenericServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Generic {
	c := genericpb.NewGenericServiceClient(conn)
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
	gclient := genericpb.NewGenericServiceClient(conn)
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}

	resp, err := gclient.DoCommand(ctx, &commonpb.DoCommandRequest{
		Name:    name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}

// Package generic contains a gRPC based generic client.
package generic

import (
	"context"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
)

// client implements GenericServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	name   string
	client genericpb.GenericServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (resource.Resource, error) {
	c := genericpb.NewGenericServiceClient(conn)
	return &client{
		Named:  name.AsNamed(),
		name:   name.ShortNameForClient(),
		client: c,
		logger: logger,
	}, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.DoCommand(ctx, &commonpb.DoCommandRequest{
		Name:    c.name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}

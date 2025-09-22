// Package button contains a gRPC based button client.
package button

import (
	"context"

	pb "go.viam.com/api/component/button/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements GripperServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.ButtonServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Button, error) {
	c := pb.NewButtonServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: c,
		logger: logger,
	}, nil
}

func (c *client) Push(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Push(ctx, &pb.PushRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

// Package toggleswitch contains a gRPC based switch client.
package toggleswitch

import (
	"context"
	"errors"

	pb "go.viam.com/api/component/switch/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements SwitchServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.SwitchServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Switch, error) {
	c := pb.NewSwitchServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: c,
		logger: logger,
	}, nil
}

func (c *client) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.SetPosition(ctx, &pb.SetPositionRequest{
		Name:     c.name,
		Position: position,
		Extra:    ext,
	})
	return err
}

func (c *client) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	resp, err := c.client.GetPosition(ctx, &pb.GetPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return 0, err
	}
	return resp.Position, nil
}

func (c *client) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, nil, err
	}
	resp, err := c.client.GetNumberOfPositions(ctx, &pb.GetNumberOfPositionsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return 0, nil, err
	}
	if len(resp.Labels) > 0 && len(resp.Labels) != int(resp.NumberOfPositions) {
		return 0, nil, errors.New("the number of labels does not match the number of positions")
	}
	return resp.NumberOfPositions, resp.Labels, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

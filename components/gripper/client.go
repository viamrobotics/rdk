// Package gripper contains a gRPC based gripper client.
package gripper

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/referenceframe"
)

// client implements GripperServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.GripperServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Gripper {
	c := pb.NewGripperServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

func (c *client) Open(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Open(ctx, &pb.OpenRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	resp, err := c.client.Grab(ctx, &pb.GrabRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

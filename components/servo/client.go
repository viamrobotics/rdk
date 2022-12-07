// Package servo contains a gRPC bases servo client
package servo

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/servo/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
)

// client implements ServoServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.ServoServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Servo {
	c := pb.NewServoServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

func (c *client) Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.MoveRequest{AngleDeg: angleDeg, Name: c.name, Extra: ext}
	if _, err := c.client.Move(ctx, req); err != nil {
		return err
	}
	return nil
}

func (c *client) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	req := &pb.GetPositionRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.PositionDeg, nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{Name: c.name, Extra: ext})
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

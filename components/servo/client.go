// Package servo contains a gRPC bases servo client
package servo

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	pb "go.viam.com/rdk/proto/api/component/servo/v1"
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

func (c *client) Move(ctx context.Context, angleDeg uint8) error {
	req := &pb.MoveRequest{AngleDeg: uint32(angleDeg), Name: c.name}
	if _, err := c.client.Move(ctx, req); err != nil {
		return err
	}
	return nil
}

func (c *client) GetPosition(ctx context.Context) (uint8, error) {
	req := &pb.GetPositionRequest{Name: c.name}
	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return 0, err
	}
	return uint8(resp.PositionDeg), nil
}

func (c *client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &pb.StopRequest{Name: c.name})
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

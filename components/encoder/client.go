package encoder

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
)

// client implements EncoderServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.EncoderServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Encoder {
	c := pb.NewEncoderServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

// GetPosition returns number of ticks since last zeroing.
func (c *client) GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return 0, err
	}
	req := &pb.GetPositionRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.GetPosition(), nil
}

// ResetPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (c *client) ResetPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return err
	}
	req := &pb.ResetPositionRequest{Name: c.name, Offset: offset, Extra: ext}
	_, err = c.client.ResetPosition(ctx, req)
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

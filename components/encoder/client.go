package encoder

import (
	"context"

	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements EncoderServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	conn   rpc.ClientConn
	client pb.EncoderServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Encoder, error) {
	c := pb.NewEncoderServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		conn:   conn,
		client: c,
		logger: logger,
	}, nil
}

// Position returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (c *client) Position(
	ctx context.Context,
	positionType PositionType,
	extra map[string]interface{},
) (float64, PositionType, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return 0, PositionTypeUnspecified, err
	}
	posType := ToProtoPositionType(positionType)
	req := &pb.GetPositionRequest{Name: c.name, PositionType: &posType, Extra: ext}
	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return 0, PositionTypeUnspecified, err
	}
	posType1 := ToEncoderPositionType(&resp.PositionType)
	return float64(resp.Value), posType1, nil
}

// ResetPosition sets the current position of
// the encoder to be its new zero position.
func (c *client) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return err
	}
	req := &pb.ResetPositionRequest{Name: c.name, Extra: ext}
	_, err = c.client.ResetPosition(ctx, req)
	return err
}

// Properties returns a list of all the position types that are supported by a given encoder.
func (c *client) Properties(ctx context.Context, extra map[string]interface{}) (Properties, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return Properties{}, err
	}
	req := &pb.GetPropertiesRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetProperties(ctx, req)
	if err != nil {
		return Properties{}, err
	}
	return ProtoFeaturesToProperties(resp), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

// Package sensor contains a gRPC based sensor client.
package sensor

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/sensor/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements SensorServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.SensorServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger golog.Logger,
) (Sensor, error) {
	c := pb.NewSensorServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: c,
		logger: logger,
	}, nil
}

func (c *client) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetReadings(ctx, &pb.GetReadingsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}

	return protoutils.ReadingProtoToGo(resp.Readings)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

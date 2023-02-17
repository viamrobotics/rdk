// Package sensor contains a gRPC based sensor client.
package sensor

import (
	"context"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/sensor/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
)

// client implements SensorServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.SensorServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Sensor {
	c := pb.NewSensorServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
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
	command, err := vprotoutils.StructToStructPb(cmd)
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

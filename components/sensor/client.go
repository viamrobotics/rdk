// Package sensor contains a gRPC based sensor client.
package sensor

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/sensor/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
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

func (c *client) Readings(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.GetReadings(ctx, &pb.GetReadingsRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}

	return protoutils.ReadingProtoToGo(resp.Readings)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

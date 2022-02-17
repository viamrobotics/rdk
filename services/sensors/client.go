// Package sensors contains a gRPC based object manipulation client
package sensors

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is a client satisfies the sensors.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.SensorsServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewSensorsServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.conn.Close()
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Service, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) GetSensors(ctx context.Context) ([]resource.Name, error) {
	resp, err := c.client.GetSensors(ctx, &pb.SensorsServiceGetSensorsRequest{})
	if err != nil {
		return nil, err
	}
	sensors := make([]resource.Name, len(resp.Sensors))
	for _, name := range resp.Sensors {
		sensors = append(sensors, protoutils.ProtoToResourceName(name))
	}
	return sensors, nil
}

func (c *client) GetReadings(ctx context.Context, sensors []resource.Name) ([]Reading, error) {
	names := make([]*commonpb.ResourceName, len(sensors))
	for _, name := range sensors {
		names = append(names, protoutils.ResourceNameToProto(name))
	}
	resp, err := c.client.GetReadings(ctx, &pb.SensorsServiceGetReadingsRequest{Sensors: names})
	if err != nil {
		return nil, err
	}
	readings := make([]Reading, len(resp.Readings))
	for _, reading := range resp.Readings {
		sReading := make([]interface{}, 0, len(reading.Readings))
		for _, r := range reading.Readings {
			sReading = append(sReading, r.AsInterface())
		}
		readings = append(
			readings, Reading{
				Name:    protoutils.ProtoToResourceName(reading.Name),
				Reading: sReading,
			})
	}
	return readings, nil
}

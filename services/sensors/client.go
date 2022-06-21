// Package sensors contains a gRPC based sensors service client
package sensors

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/sensors/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is a client implements the SensorsServiceClient.
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
	return nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) GetSensors(ctx context.Context) ([]resource.Name, error) {
	resp, err := c.client.GetSensors(ctx, &pb.GetSensorsRequest{})
	if err != nil {
		return nil, err
	}
	sensorNames := make([]resource.Name, 0, len(resp.SensorNames))
	for _, name := range resp.SensorNames {
		sensorNames = append(sensorNames, protoutils.ResourceNameFromProto(name))
	}
	return sensorNames, nil
}

func (c *client) GetReadings(ctx context.Context, sensorNames []resource.Name) ([]Readings, error) {
	names := make([]*commonpb.ResourceName, 0, len(sensorNames))
	for _, name := range sensorNames {
		names = append(names, protoutils.ResourceNameToProto(name))
	}

	resp, err := c.client.GetReadings(ctx, &pb.GetReadingsRequest{SensorNames: names})
	if err != nil {
		return nil, err
	}

	readings := make([]Readings, 0, len(resp.Readings))
	for _, reading := range resp.Readings {
		sReading := make([]interface{}, 0, len(reading.Readings))
		for _, r := range reading.Readings {
			sReading = append(sReading, r.AsInterface())
		}
		readings = append(
			readings, Readings{
				Name:     protoutils.ResourceNameFromProto(reading.Name),
				Readings: sReading,
			})
	}
	return readings, nil
}

// Package sensors contains a gRPC based sensors service client
package sensors

import (
	"context"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/sensors/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements SensorsServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.SensorsServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewSensorsServiceClient(conn)
	c := &client{
		name:   name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c
}

func (c *client) Sensors(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetSensors(ctx, &pb.GetSensorsRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	sensorNames := make([]resource.Name, 0, len(resp.SensorNames))
	for _, name := range resp.SensorNames {
		sensorNames = append(sensorNames, rprotoutils.ResourceNameFromProto(name))
	}
	return sensorNames, nil
}

func (c *client) Readings(ctx context.Context, sensorNames []resource.Name, extra map[string]interface{}) ([]Readings, error) {
	names := make([]*commonpb.ResourceName, 0, len(sensorNames))
	for _, name := range sensorNames {
		names = append(names, rprotoutils.ResourceNameToProto(name))
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetReadings(ctx, &pb.GetReadingsRequest{Name: c.name, SensorNames: names, Extra: ext})
	if err != nil {
		return nil, err
	}

	readings := make([]Readings, 0, len(resp.Readings))
	for _, reading := range resp.Readings {
		sReading, err := rprotoutils.ReadingProtoToGo(reading.Readings)
		if err != nil {
			return nil, err
		}
		readings = append(
			readings, Readings{
				Name:     rprotoutils.ResourceNameFromProto(reading.Name),
				Readings: sReading,
			})
	}
	return readings, nil
}

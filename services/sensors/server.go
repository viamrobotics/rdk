// Package sensors contains a gRPC based sensors service server
package sensors

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/sensors/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the SensorsService from sensors.proto.
type subtypeServer struct {
	pb.UnimplementedSensorsServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a sensors gRPC service server.
func NewServer(s subtype.Service) pb.SensorsServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetSensors(
	ctx context.Context,
	req *pb.GetSensorsRequest,
) (*pb.GetSensorsResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	names, err := svc.Sensors(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	sensorNames := make([]*commonpb.ResourceName, 0, len(names))
	for _, name := range names {
		sensorNames = append(sensorNames, protoutils.ResourceNameToProto(name))
	}

	return &pb.GetSensorsResponse{SensorNames: sensorNames}, nil
}

func (server *subtypeServer) GetReadings(
	ctx context.Context,
	req *pb.GetReadingsRequest,
) (*pb.GetReadingsResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	sensorNames := make([]resource.Name, 0, len(req.SensorNames))
	for _, name := range req.SensorNames {
		sensorNames = append(sensorNames, protoutils.ResourceNameFromProto(name))
	}

	readings, err := svc.Readings(ctx, sensorNames, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	readingsP := make([]*pb.Readings, 0, len(readings))
	for _, reading := range readings {
		rReading, err := protoutils.ReadingGoToProto(reading.Readings)
		if err != nil {
			return nil, err
		}
		readingP := &pb.Readings{
			Name:     protoutils.ResourceNameToProto(reading.Name),
			Readings: rReading,
		}
		readingsP = append(readingsP, readingP)
	}

	return &pb.GetReadingsResponse{Readings: readingsP}, nil
}

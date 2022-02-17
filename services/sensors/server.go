// Package sensors contains a gRPC based sensors service server
package sensors

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the contract from sensors.proto.
type subtypeServer struct {
	pb.UnimplementedSensorsServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a sensors gRPC service server.
func NewServer(s subtype.Service) pb.SensorsServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("sensors.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetSensors(
	ctx context.Context,
	req *pb.SensorsServiceGetSensorsRequest,
) (*pb.SensorsServiceGetSensorsResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	names, err := svc.GetSensors(ctx)
	if err != nil {
		return nil, err
	}
	sensors := make([]*commonpb.ResourceName, 0, len(names))
	for _, name := range names {
		sensors = append(sensors, protoutils.ResourceNameToProto(name))
	}

	return &pb.SensorsServiceGetSensorsResponse{Sensors: sensors}, nil
}

func (server *subtypeServer) GetReadings(
	ctx context.Context,
	req *pb.SensorsServiceGetReadingsRequest,
) (*pb.SensorsServiceGetReadingsResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	sensors := make([]resource.Name, 0, len(req.Sensors))
	for _, name := range req.Sensors {
		sensors = append(sensors, protoutils.ProtoToResourceName(name))
	}

	readings, err := svc.GetReadings(ctx, sensors)
	if err != nil {
		return nil, err
	}

	readingsP := make([]*pb.Reading, 0, len(readings))
	for _, reading := range readings {
		rReading := make([]*structpb.Value, 0, len(reading.Reading))
		for _, r := range reading.Reading {
			v, err := structpb.NewValue(r)
			if err != nil {
				return nil, err
			}
			rReading = append(rReading, v)
		}
		readingP := &pb.Reading{
			Name:     protoutils.ResourceNameToProto(reading.Name),
			Readings: rReading,
		}
		readingsP = append(readingsP, readingP)
	}

	return &pb.SensorsServiceGetReadingsResponse{Readings: readingsP}, nil
}

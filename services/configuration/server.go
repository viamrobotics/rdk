package configuration

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/configuration/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the contract from configuration.proto.
type subtypeServer struct {
	pb.UnimplementedConfigurationServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a framesystem gRPC service server.
func NewServer(s subtype.Service) pb.ConfigurationServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("configuration.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetCameras(ctx context.Context, req *pb.GetCamerasRequest) (
	*pb.GetCamerasResponse, error,
) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	cameras, err := svc.GetCameras(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetCamerasResponse{Cameras: cameras}, nil
}

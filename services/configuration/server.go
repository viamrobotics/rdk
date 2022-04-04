package configuration

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/configuration/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the contract from configuration.proto.
type subtypeServer struct {
	pb.UnimplementedNavigationServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a framesystem gRPC service server.
func NewServer(s subtype.Service) pb.NavigationServiceServer {
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

func (server *subtypeServer) DiscoverCameras(ctx context.Context, req *pb.DiscoverCamerasRequest) (
	*pb.DiscoverCamerasResponse, error,
) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	cameras, err := svc.DiscoverCameras(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetModeResponse{DiscoverCameras: cameras}, nil
}

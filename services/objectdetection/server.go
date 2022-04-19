package objectdetection

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/objectdetection/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the Object Detection Service.
type subtypeServer struct {
	pb.UnimplementedObjectDetectionServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a object detection gRPC service server.
func NewServer(s subtype.Service) pb.ObjectDetectionServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectdetection.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) DetectorNames(
	ctx context.Context,
	req *pb.DetectorNamesRequest,
) (*pb.DetectorNamesResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	names, err := svc.DetectorNames(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.DetectorNamesResponse{
		DetectorNames: names,
	}, nil
}

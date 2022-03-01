// Package objectmanipulation contains a gRPC based object manipulation service server
package objectmanipulation

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/objectmanipulation/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the ObjectManipulationService from object_manipulation.proto.
type subtypeServer struct {
	pb.UnimplementedObjectManipulationServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a object manipulation gRPC service server.
func NewServer(s subtype.Service) pb.ObjectManipulationServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectmanipulation.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) DoGrab(ctx context.Context, req *pb.DoGrabRequest) (*pb.DoGrabResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	obstacles := req.GetWorldState().GetObstacles()
	geometriesInFrames := make([]*referenceframe.GeometriesInFrame, len(obstacles))
	for i, geometryInFrame := range obstacles {
		geometriesInFrames[i], err = referenceframe.ProtobufToGeometriesInFrame(geometryInFrame)
		if err != nil {
			return nil, err
		}
	}
	success, err := svc.DoGrab(ctx, req.GetGripperName(), referenceframe.ProtobufToPoseInFrame(req.GetTarget()), geometriesInFrames)
	if err != nil {
		return nil, err
	}
	return &pb.DoGrabResponse{Success: success}, nil
}

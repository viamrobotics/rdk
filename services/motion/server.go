// Package motion contains a gRPC based motion service server
package motion

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the MotionService from motion.proto.
type subtypeServer struct {
	pb.UnimplementedMotionServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a motion gRPC service server.
func NewServer(s subtype.Service) pb.MotionServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("motion.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
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
	success, err := svc.Move(ctx, req.GetComponentName(), referenceframe.ProtobufToPoseInFrame(req.GetDestination()), geometriesInFrames)
	if err != nil {
		return nil, err
	}
	return &pb.MoveResponse{Success: success}, nil
}

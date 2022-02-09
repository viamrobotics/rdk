// Package objectmanipulation contains a gRPC based object manipulation service server
package objectmanipulation

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from object_manipulation.proto.
type subtypeServer struct {
	pb.UnimplementedObjectManipulationServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a object manipulation gRPC service server.
func NewServer(s subtype.Service) pb.ObjectManipulationServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	name := Name
	resource := server.subtypeSvc.Resource(name.String())
	if resource == nil {
		return nil, errors.Errorf("no resource with name (%s)", name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, errors.Errorf(
			"resource with name (%s) is not an object manipulation service", name)
	}
	return svc, nil
}

func (server *subtypeServer) DoGrab(
	ctx context.Context,
	req *pb.ObjectManipulationServiceDoGrabRequest,
) (*pb.ObjectManipulationServiceDoGrabResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	success, err := svc.DoGrab(
		ctx, req.GetGripperName(), req.GetArmName(), req.GetCameraName(), protoToVector(req.GetCameraPoint()),
	)
	if err != nil {
		return nil, err
	}
	return &pb.ObjectManipulationServiceDoGrabResponse{Success: success}, nil
}

func protoToVector(p *commonpb.Vector3) *r3.Vector {
	return &r3.Vector{
		X: p.X,
		Y: p.Y,
		Z: p.Z,
	}
}

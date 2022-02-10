// Package framesystem contains a gRPC based frame system service server
package framesystem

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from frame_system.proto.
type subtypeServer struct {
	pb.UnimplementedFrameSystemServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a framesystem gRPC service server.
func NewServer(s subtype.Service) pb.FrameSystemServiceServer {
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
			"resource with name (%s) is not a frame system service", name)
	}
	return svc, nil
}

func (server *subtypeServer) Config(
	ctx context.Context,
	req *pb.FrameSystemServiceConfigRequest,
) (*pb.FrameSystemServiceConfigResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	sortedParts, err := svc.Config(ctx)
	if err != nil {
		return nil, err
	}
	configs := make([]*pb.FrameSystemServiceConfig, len(sortedParts))
	for i, part := range sortedParts {
		c, err := part.ToProtobuf()
		if err != nil {
			if errors.Is(err, referenceframe.ErrNoModelInformation) {
				configs[i] = nil
				continue
			}
			return nil, err
		}
		configs[i] = c
	}
	return &pb.FrameSystemServiceConfigResponse{FrameSystemConfigs: configs}, nil
}

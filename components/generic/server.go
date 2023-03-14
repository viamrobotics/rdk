// Package generic contains a gRPC based generic service subtypeServer.
package generic

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the generic.Generic service.
type subtypeServer struct {
	genericpb.UnimplementedGenericServiceServer
	s subtype.Service
}

// NewServer constructs an generic gRPC service subtypeServer.
func NewServer(s subtype.Service) genericpb.GenericServiceServer {
	return &subtypeServer{s: s}
}

// getGeneric returns the component specified, nil if not.
func (s *subtypeServer) getGeneric(name string) (Generic, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no resource with name (%s)", name)
	}
	generic, ok := resource.(Generic)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a generic component", name)
	}
	return generic, nil
}

// DoCommand returns an arbitrary command and returns arbitrary results.
func (s *subtypeServer) DoCommand(ctx context.Context, req *commonpb.DoCommandRequest) (*commonpb.DoCommandResponse, error) {
	genericDevice, err := s.getGeneric(req.Name)
	if err != nil {
		return nil, err
	}
	result, err := genericDevice.DoCommand(ctx, req.Command.AsMap())
	if err != nil {
		return nil, err
	}
	res, err := protoutils.StructToStructPb(result)
	if err != nil {
		return nil, err
	}
	return &commonpb.DoCommandResponse{Result: res}, nil
}

// Package generic contains a gRPC based generic service serviceServer.
package generic

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/service/generic/v1"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the resource.Generic service.
type serviceServer struct {
	genericpb.UnimplementedGenericServiceServer
	coll resource.APIResourceGetter[resource.Resource]
}

// NewRPCServiceServer constructs an generic gRPC service serviceServer.
func NewRPCServiceServer(coll resource.APIResourceGetter[resource.Resource], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// DoCommand returns an arbitrary command and returns arbitrary results.
func (s *serviceServer) DoCommand(ctx context.Context, req *commonpb.DoCommandRequest) (*commonpb.DoCommandResponse, error) {
	genericDevice, err := s.coll.Resource(req.Name)
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

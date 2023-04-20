// Package generic contains a gRPC based generic service subtypeServer.
package generic

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
)

// subtypeServer implements the resource.Generic service.
type subtypeServer struct {
	genericpb.UnimplementedGenericServiceServer
	coll resource.SubtypeCollection[resource.Resource]
}

// NewRPCServiceServer constructs an generic gRPC service subtypeServer.
func NewRPCServiceServer(coll resource.SubtypeCollection[resource.Resource]) interface{} {
	return &subtypeServer{coll: coll}
}

// DoCommand returns an arbitrary command and returns arbitrary results.
func (s *subtypeServer) DoCommand(ctx context.Context, req *commonpb.DoCommandRequest) (*commonpb.DoCommandResponse, error) {
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

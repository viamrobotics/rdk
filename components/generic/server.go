// Package generic contains a gRPC based generic service serviceServer.
package generic

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/utils/protoutils"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
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
		return nil, errtrace.Wrap(err)
	}
	result, err := genericDevice.DoCommand(ctx, req.Command.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	res, err := protoutils.StructToStructPb(result)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &commonpb.DoCommandResponse{Result: res}, nil
}

// GetStatus returns the status of the generic component.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(rprotoutils.GetStatusFromResourceServer(ctx, res, req))
}

// GetGeometries returns the geometries of the generic component in their current configuration. Generic component
// implementations may opt in to providing geometries by implementing [resource.Shaped]; otherwise, an empty list is returned.
func (s *serviceServer) GetGeometries(
	ctx context.Context,
	req *commonpb.GetGeometriesRequest,
) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	shaped, ok := res.(resource.Shaped)
	if !ok {
		return &commonpb.GetGeometriesResponse{}, nil
	}
	geometries, err := shaped.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}

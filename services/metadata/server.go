// Package metadata contains a gRPC based metadata service server.
package metadata

import (
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// Server implements the contract from metadata.proto.
type Server struct {
	pb.UnimplementedMetadataServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a gRPC metadata server.
func NewServer(s subtype.Service) pb.MetadataServiceServer {
	return &Server{subtypeSvc: s}
}

// NewServerFromMap constructs a subtype.Service from the provided map and uses
// that to construct a gRPC metadata server.
func NewServerFromMap(subtypeSvcMap map[resource.Name]interface{}) (pb.MetadataServiceServer, error) {
	subtypeSvc, err := subtype.New(subtypeSvcMap)
	if err != nil {
		return nil, err
	}
	return NewServer(subtypeSvc), nil
}

func (server *Server) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("metadata.Service", resource)
	}

	return svc, nil
}

// Resources returns the list of resources from the Server.
func (server *Server) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}

	all := svc.Resources(ctx)
	rNames := make([]*commonpb.ResourceName, 0, len(all))
	for _, m := range all {
		rNames = append(
			rNames,
			protoutils.ResourceNameToProto(m),
		)
	}
	return &pb.ResourcesResponse{Resources: rNames}, nil
}

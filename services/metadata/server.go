// Package metadata contains a gRPC based metadata service server.
package metadata

import (
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// server implements the contract from metadata.proto.
type server struct {
	pb.UnimplementedMetadataServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a gRPC metadata server.
func NewServer(s subtype.Service) pb.MetadataServiceServer {
	return &server{subtypeSvc: s}
}

func (server *server) service() (Service, error) {
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
func (server *server) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}

	all, err := svc.Resources(ctx)
	if err != nil {
		return nil, err
	}

	rNames := make([]*commonpb.ResourceName, 0, len(all))
	for _, m := range all {
		rNames = append(
			rNames,
			protoutils.ResourceNameToProto(m),
		)
	}
	return &pb.ResourcesResponse{Resources: rNames}, nil
}

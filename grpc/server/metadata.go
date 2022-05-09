// Package server contains a gRPC based metadata service server.
package server

import (
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/robot/metadata"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// server implements the contract from metadata.proto.
type server struct {
	pb.UnimplementedMetadataServiceServer
	subtypeSvc subtype.Service
}

// NewMetadataServer constructs a gRPC metadata server.
func NewMetadataServer(s subtype.Service) pb.MetadataServiceServer {
	return &server{subtypeSvc: s}
}

// CR erodkin: this will need some work since we want to dump metadata.Name probably.
func (server *server) service() (metadata.Service, error) {
	resource := server.subtypeSvc.Resource(metadata.Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(metadata.Name)
	}
	svc, ok := resource.(metadata.Service)
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

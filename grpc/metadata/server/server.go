// Package server contains a gRPC based metadata service server.
package server

import (
	"context"

	"go.viam.com/rdk/metadata/service"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/protoutils"
)

// MetadataServer implements the contract from metadata.proto.
type MetadataServer struct {
	pb.UnimplementedMetadataServiceServer
	s service.Metadata
}

// New constructs a gRPC service server.
func New(s service.Metadata) pb.MetadataServiceServer {
	return &MetadataServer{s: s}
}

// Resources returns the list of resources.
func (s *MetadataServer) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	rNames := make([]*commonpb.ResourceName, 0, len(s.s.All()))
	for _, m := range s.s.All() {
		rNames = append(
			rNames,
			protoutils.ResourceNameToProto(m),
		)
	}
	return &pb.ResourcesResponse{Resources: rNames}, nil
}

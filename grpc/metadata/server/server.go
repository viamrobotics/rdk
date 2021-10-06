// Package server contains a gRPC based metadata service server.
package server

import (
	"context"

	"go.viam.com/core/metadata/service"
	pb "go.viam.com/core/proto/api/service/v1"
)

// MetadataServer implements the contract from metadata.proto
type MetadataServer struct {
	pb.UnimplementedMetadataServiceServer
	s *service.Service
}

// New constructs a gRPC service server.
func New(s *service.Service) pb.MetadataServiceServer {
	return &MetadataServer{s: s}
}

// Resources returns the list of resources.
func (s *MetadataServer) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	rNames := make([]*pb.ResourceName, 0, len(s.s.All()))
	for _, m := range s.s.All() {
		rNames = append(
			rNames,
			&pb.ResourceName{
				Uuid:      m.UUID,
				Namespace: string(m.Namespace),
				Type:      string(m.ResourceType),
				Subtype:   string(m.ResourceSubtype),
				Name:      m.Name,
			},
		)
	}
	return &pb.ResourcesResponse{Resources: rNames}, nil
}

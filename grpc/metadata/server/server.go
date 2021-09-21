// Package server contains a gRPC based metadata service server.
package server

import (
	"context"

	"go.viam.com/core/metadata"
	pb "go.viam.com/core/proto/api/service/v1"
)

// MetadataServer implements the contract from metadata.proto
type MetadataServer struct {
	pb.UnimplementedMetadataServiceServer
	m *metadata.Metadata
}

// New constructs a gRPC service server.
func New(m *metadata.Metadata) pb.MetadataServiceServer {
	return &MetadataServer{m: m}
}

// Resources returns the list of resources.
func (s *MetadataServer) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	rNames := make([]*pb.ResourceName, 0, len(s.m.All()))
	for _, m := range s.m.All() {
		rNames = append(
			rNames,
			&pb.ResourceName{
				Uuid:      m.UUID,
				Namespace: m.Namespace,
				Type:      m.Type,
				Subtype:   m.Subtype,
				Name:      m.Name,
			},
		)
	}
	return &pb.ResourcesResponse{Resources: rNames}, nil
}

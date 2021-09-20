// Package server contains a gRPC based metadata service server.
package server

import (
	"context"

	pb "go.viam.com/core/proto/api/service/v1"
	"go.viam.com/core/resources"
)

// MetadataServer implements the contract from metadata.proto
type MetadataServer struct {
	pb.UnimplementedMetadataServiceServer
	r *resources.Resources
}

// New constructs a gRPC service server.
func New(r *resources.Resources) pb.MetadataServiceServer {
	return &MetadataServer{r: r}
}

// Resources returns the list of resources.
func (s *MetadataServer) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	rNames := make([]*pb.ResourceName, 0, len(s.r.GetResources()))
	for _, r := range s.r.GetResources() {
		rNames = append(
			rNames,
			&pb.ResourceName{
				Uuid:      r.UUID,
				Namespace: r.Namespace,
				Type:      r.Type,
				Subtype:   r.Subtype,
				Name:      r.Name,
			},
		)
	}
	return &pb.ResourcesResponse{Resources: rNames}, nil
}

// Package grpchelper implements helper functions to be used with slam service grpc clients
package grpchelper

import (
	"context"

	pb "go.viam.com/api/service/slam/v1"
)

// GetPointCloudMapStreamCallback helps a client request the point cloud stream from a SLAM server,
// returning a callback function for accessing the stream data.
func GetPointCloudMapStreamCallback(ctx context.Context, name string, slamClient pb.SLAMServiceClient) (func() ([]byte, error), error) {
	req := &pb.GetPointCloudMapStreamRequest{Name: name}

	resp, err := slamClient.GetPointCloudMapStream(ctx, req)
	// If there is an issue with the SLAM algo but a gRPC server is present, the stream client returned will not
	// fail until data is requested
	if err != nil {
		return nil, err
	}

	f := func() ([]byte, error) {
		chunk, err := resp.Recv()
		if err != nil {
			return nil, err
		}

		return chunk.GetPointCloudPcdChunk(), err
	}

	return f, nil
}

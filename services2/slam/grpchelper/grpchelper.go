// Package grpchelper implements helper functions to be used with slam service grpc clients
package grpchelper

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/slam/v1"
)

// PointCloudMapCallback helps a client request the point cloud stream from a SLAM server,
// returning a callback function for accessing the stream data.
func PointCloudMapCallback(ctx context.Context, name string, slamClient pb.SLAMServiceClient) (func() ([]byte, error), error) {
	req := &pb.GetPointCloudMapRequest{Name: name}

	// If the target gRPC server returns an error status, this call doesn't return an error.
	// Instead, the error status will be returned to the first call to resp.Recv().
	// This call only returns an error if the connection to the target gRPC server can't be established, is canceled, etc.
	resp, err := slamClient.GetPointCloudMap(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting the pointcloud map from the SLAM client")
	}

	f := func() ([]byte, error) {
		chunk, err := resp.Recv()
		if err != nil {
			return nil, errors.Wrap(err, "error receiving pointcloud chunk")
		}

		return chunk.GetPointCloudPcdChunk(), err
	}

	return f, nil
}

// InternalStateCallback helps a client request the internal state stream from a SLAM server,
// returning a callback function for accessing the stream data.
func InternalStateCallback(ctx context.Context, name string, slamClient pb.SLAMServiceClient) (func() ([]byte, error), error) {
	req := &pb.GetInternalStateRequest{Name: name}

	// If the target gRPC server returns an error status, this call doesn't return an error.
	// Instead, the error status will be returned to the first call to resp.Recv().
	// This call only returns an error if the connection to the target gRPC server can't be established, is canceled, etc.
	resp, err := slamClient.GetInternalState(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting the internal state from the SLAM client")
	}

	f := func() ([]byte, error) {
		chunk, err := resp.Recv()
		if err != nil {
			return nil, errors.Wrap(err, "error receiving internal state chunk")
		}

		return chunk.GetInternalStateChunk(), nil
	}
	return f, err
}

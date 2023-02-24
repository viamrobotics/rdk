// Package grpchelper implements helper functions to be used with slam service grpc clients
package grpchelper

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/slam/v1"
)

// HelperGetInternalStateCallback helps a client request the internal state stream from a SLAM server.
func HelperGetInternalStateCallback(ctx context.Context, name string, slamClient pb.SLAMServiceClient) (func() ([]byte, error), error) {
	req := &pb.GetInternalStateStreamRequest{Name: name}

	resp, err := slamClient.GetInternalStateStream(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting the internal state from the SLAM client")
	}

	f := func() ([]byte, error) {
		chunk, err := resp.Recv()
		if err != nil {
			return nil, err
		}

		return chunk.GetInternalStateChunk(), nil
	}
	return f, err
}

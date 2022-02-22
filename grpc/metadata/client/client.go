// Package client contains a gRPC based metadata service client.
package client

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
)

// MetadataServiceClient is a client satisfies the metadata.proto contract.
type MetadataServiceClient struct {
	conn   rpc.ClientConn
	client pb.MetadataServiceClient

	logger golog.Logger
}

// New constructs a new MetadataServiceClient that is served at the given address.
func New(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*MetadataServiceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}

	client := pb.NewMetadataServiceClient(conn)
	mc := &MetadataServiceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return mc, nil
}

// Close cleanly closes the underlying connections.
func (mc *MetadataServiceClient) Close() error {
	return mc.conn.Close()
}

// Resources either gets the latest version of the list of resources for the remote robot.
func (mc *MetadataServiceClient) Resources(ctx context.Context) ([]*commonpb.ResourceName, error) {
	resp, err := mc.client.Resources(ctx, &pb.ResourcesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Resources, nil
}

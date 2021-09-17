// Package client contains a gRPC based metadata service client.
package client

import (
	"context"

	rpcclient "go.viam.com/utils/rpc/client"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"

	pb "go.viam.com/core/proto/api/service/v1"
)

// MetadataServiceClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type MetadataServiceClient struct {
	address string
	conn    dialer.ClientConn
	client  pb.MetadataServiceClient

	logger golog.Logger
}

func NewClient(ctx context.Context, address string, logger golog.Logger) (*MetadataServiceClient, error) {
	conn, err := rpcclient.Dial(ctx, address, rpcclient.DialOptions{Insecure: true}, logger)
	if err != nil {
		return nil, err
	}

	client := pb.NewMetadataServiceClient(conn)
	mc := &MetadataServiceClient{
		address: address,
		conn:    conn,
		client:  client,
		logger:  logger,
	}
	return mc, nil
}

// Close cleanly closes the underlying connections
func (mc *MetadataServiceClient) Close() error {
	return mc.conn.Close()
}

// Resources either gets a cached or latest version of the status of the remote
// robot.
func (mc *MetadataServiceClient) Resources(ctx context.Context) ([]*pb.ResourceName, error) {
	resp, err := mc.client.Resources(ctx, &pb.ResourcesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Resources, nil
}

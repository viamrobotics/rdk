// Package metadata contains a gRPC based metadata service client.
package metadata

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
)

// ServiceClient is a client satisfies the metadata.proto contract.
type ServiceClient struct {
	conn   rpc.ClientConn
	client pb.MetadataServiceClient

	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *ServiceClient {
	client := pb.NewMetadataServiceClient(conn)
	mc := &ServiceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return mc
}

// NewClient constructs a new ServiceClient that is served at the given address.
func NewClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*ServiceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}

	mc := newSvcClientFromConn(conn, logger)
	return mc, nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(_ctx context.Context, conn rpc.ClientConn, _name string, logger golog.Logger) *ServiceClient {
	return newSvcClientFromConn(conn, logger)
}

// Close cleanly closes the underlying connections.
func (mc *ServiceClient) Close() error {
	return mc.conn.Close()
}

// Resources either gets the latest version of the list of resources for the remote robot.
func (mc *ServiceClient) Resources(ctx context.Context) ([]*commonpb.ResourceName, error) {
	resp, err := mc.client.Resources(ctx, &pb.ResourcesRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Resources, nil
}

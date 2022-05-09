// Package client contains a gRPC based metadata service client.
// CR erodkin: probably we should get rid of this module entirely, move
// relevant functionality into client.go
package client

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/metadata"
)

// client is a client satisfies the MetadataServiceClient.
type client struct {
	conn   rpc.ClientConn
	client pb.MetadataServiceClient

	logger golog.Logger
}

// newSvcClientFromConn constructs a new client using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) metadata.Service {
	mc := &client{
		conn:   conn,
		client: pb.NewMetadataServiceClient(conn),
		logger: logger,
	}
	return mc
}

// NewMetadataClient constructs a new client that is served at the given address.
func NewMetadataClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (metadata.Service, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}

	mc := newSvcClientFromConn(conn, logger)
	return mc, nil
}

// Close cleanly closes the underlying connections.
func (mc *client) Close() error {
	return mc.conn.Close()
}

// Resources gets the latest version of the list of resources for the remote robot.
func (mc *client) Resources(ctx context.Context) ([]resource.Name, error) {
	resp, err := mc.client.Resources(ctx, &pb.ResourcesRequest{})
	if err != nil {
		return nil, err
	}

	resources := make([]resource.Name, 0, len(resp.Resources))

	for _, name := range resp.Resources {
		newName := protoutils.ResourceNameFromProto(name)
		resources = append(resources, newName)
	}
	return resources, nil
}

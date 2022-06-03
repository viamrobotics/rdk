// Package motion contains a gRPC based motion client
package datamanager

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/datamanager/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is a client satisfies the data_manager.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.DataManagerServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewDataManagerServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.conn.Close()
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (dataManagerService, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) dataManagerService {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) Sync(
	ctx context.Context,
	name resource.Name,
) (bool, error) {
	resp, err := c.client.Sync(ctx, &pb.SyncRequest{
		Name: protoutils.ResourceNameToProto(name),
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

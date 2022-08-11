// Package model implements the model storage/deployment client.
package model

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/proto/viam/model/v1"
	"go.viam.com/utils/rpc"
)

// client is a client satisfies the model.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.ModelServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewModelServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// NewClientFromConn constructs a new Client from connection passed in.
//nolint:revive
func NewClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	return c.client.Delete(ctx, req)
}

func (c *client) Upload(ctx context.Context) (pb.ModelService_UploadClient, error) {
	return c.client.Upload(ctx)
}

func (c *client) Deploy(ctx context.Context, req *pb.DeployRequest) (*pb.DeployResponse, error) {
	return c.client.Deploy(ctx, req)
}

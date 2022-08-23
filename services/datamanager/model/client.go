// Package model implements model storage/deployment client.
package model

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	v1 "go.viam.com/api/proto/viam/model/v1"
)

// client implements ModelServiceClient.
type client struct {
	conn   rpc.ClientConn
	client v1.ModelServiceClient
	logger golog.Logger
}

// NewClient constructs a new pb.ModelServiceClient using the passed in connection.
func NewClient(conn rpc.ClientConn) v1.ModelServiceClient {
	fmt.Println("NewClient()")
	return v1.NewModelServiceClient(conn)
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	fmt.Println("newSvcClientFromConn()")
	grpcClient := NewClient(conn)
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
	fmt.Println("NewClientFromConn()")
	return newSvcClientFromConn(conn, logger)
}

func (c *client) Delete(ctx context.Context, req *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	return c.client.Delete(ctx, req)
}

func (c *client) Upload(ctx context.Context) (v1.ModelService_UploadClient, error) {
	return c.client.Upload(ctx)
}

func (c *client) Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error) {
	// fmt.Println("Deploy()")
	// resp, err := c.client.Deploy(ctx, req)
	// fmt.Println("err: ", err)
	return c.client.Deploy(ctx, req)
}

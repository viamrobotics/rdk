// Package gripper contains a gRPC based gripper client.
package gripper

import (
	"context"

	rpcclient "go.viam.com/utils/rpc/client"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/grpc"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/referenceframe"
)

// serviceClient is a client satisfies the gripper.proto contract.
type serviceClient struct {
	conn   dialer.ClientConn
	client pb.GripperServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, opts rpcclient.DialOptions, logger golog.Logger) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn dialer.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewGripperServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an gripper client
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, opts rpcclient.DialOptions, logger golog.Logger) (Gripper, error) {
	sc, err := newServiceClient(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(conn dialer.ClientConn, name string, logger golog.Logger) Gripper {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Gripper {
	return &client{sc, name}
}

func (c *client) Open(ctx context.Context) error {
	_, err := c.client.Open(ctx, &pb.GripperServiceOpenRequest{
		Name: c.name,
	})
	return err
}

func (c *client) Grab(ctx context.Context) (bool, error) {
	resp, err := c.client.Grab(ctx, &pb.GripperServiceGrabRequest{
		Name: c.name,
	})
	if err != nil {
		return false, err
	}
	return resp.Grabbed, nil
}

func (c *client) ModelFrame() *referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

// Close cleanly closes the underlying connections
func (c *client) Close() error {
	return c.serviceClient.Close()
}

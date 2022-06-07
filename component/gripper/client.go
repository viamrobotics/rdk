// Package gripper contains a gRPC based gripper client.
package gripper

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/gripper/v1"
	"go.viam.com/rdk/referenceframe"
)

// serviceClient is a client satisfies the gripper.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.GripperServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewGripperServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an gripper client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Gripper, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Gripper {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Gripper {
	return &client{sc, name}
}

func (c *client) Open(ctx context.Context) error {
	_, err := c.client.Open(ctx, &pb.OpenRequest{
		Name: c.name,
	})
	return err
}

func (c *client) Grab(ctx context.Context) (bool, error) {
	resp, err := c.client.Grab(ctx, &pb.GrabRequest{
		Name: c.name,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &pb.StopRequest{
		Name: c.name,
	})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

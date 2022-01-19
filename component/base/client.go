// Package base contains a gRPC based base client
package base

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// serviceClient is a client satisfies the arm.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.BaseServiceClient
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
	client := pb.NewBaseServiceClient(conn)
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

// client is a base client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Base, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Base {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Base {
	return &client{sc, name}
}

func (c *client) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	_, err := c.client.MoveStraight(ctx, &pb.BaseServiceMoveStraightRequest{
		Name:       c.name,
		DistanceMm: int64(distanceMillis),
		MmPerSec:   millisPerSec,
		Block:      block,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, degsPerSec float64, block bool) error {
	_, err := c.client.MoveArc(ctx, &pb.BaseServiceMoveArcRequest{
		Name:       c.name,
		MmPerSec:   millisPerSec,
		AngleDeg:   degsPerSec,
		DistanceMm: int64(distanceMillis),
		Block:      block,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	_, err := c.client.Spin(ctx, &pb.BaseServiceSpinRequest{
		Name:       c.name,
		AngleDeg:   angleDeg,
		DegsPerSec: degsPerSec,
		Block:      block,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &pb.BaseServiceStopRequest{Name: c.name})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) WidthGet(ctx context.Context) (int, error) {
	resp, err := c.client.WidthGet(ctx, &pb.BaseServiceWidthGetRequest{Name: c.name})
	if err != nil {
		return 0, err
	}
	return int(resp.GetWidthMm()), nil
}

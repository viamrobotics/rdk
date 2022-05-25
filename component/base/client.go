// Package base contains a gRPC based base client
package base

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
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

func (c *client) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	_, err := c.client.MoveStraight(ctx, &pb.MoveStraightRequest{
		Name:       c.name,
		DistanceMm: int64(distanceMm),
		MmPerSec:   mmPerSec,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	_, err := c.client.Spin(ctx, &pb.SpinRequest{
		Name:       c.name,
		AngleDeg:   angleDeg,
		DegsPerSec: degsPerSec,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	_, err := c.client.SetPower(ctx, &pb.SetPowerRequest{
		Name:    c.name,
		Linear:  &commonpb.Vector3{X: linear.X, Y: linear.Y, Z: linear.Z},
		Angular: &commonpb.Vector3{X: angular.X, Y: angular.Y, Z: angular.Z},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	_, err := c.client.SetVelocity(ctx, &pb.SetVelocityRequest{
		Name:    c.name,
		Linear:  &commonpb.Vector3{X: linear.X, Y: linear.Y, Z: linear.Z},
		Angular: &commonpb.Vector3{X: angular.X, Y: angular.Y, Z: angular.Z},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	_, err := c.client.SetVelocity(ctx, &pb.SetVelocityRequest{
		Name:    c.name,
		Linear:  &commonpb.Vector3{X: linear.X, Y: linear.Y, Z: linear.Z},
		Angular: &commonpb.Vector3{X: angular.X, Y: angular.Y, Z: angular.Z},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &pb.StopRequest{Name: c.name})
	if err != nil {
		return err
	}
	return nil
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

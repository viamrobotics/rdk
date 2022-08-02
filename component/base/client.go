// Package base contains a gRPC based base client
package base

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/utils"
)

// serviceClient is a client satisfies the arm.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.BaseServiceClient
	logger golog.Logger
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

// client is a base client.
type client struct {
	*serviceClient
	name string
}

type reconfigurableClient struct {
	mu     sync.RWMutex
	actual Base
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Base {
	sc := newSvcClientFromConn(conn, logger)
	base := clientFromSvcClient(sc, name)
	return &reconfigurableClient{actual: base}
}

func clientFromSvcClient(sc *serviceClient, name string) Base {
	return &client{sc, name}
}

func (c *reconfigurableClient) Reconfigure(ctx context.Context, newClient resource.Reconfigurable) error {
	client, ok := newClient.(*reconfigurableClient)
	if !ok {
		return utils.NewUnexpectedTypeError(c, newClient)
	}
	if err := viamutils.TryClose(ctx, c.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	c.actual = client.actual
	return nil
}

func (c *reconfigurableClient) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.MoveStraight(ctx, distanceMm, mmPerSec)
}

func (c *reconfigurableClient) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Spin(ctx, angleDeg, degsPerSec)
}

func (c *reconfigurableClient) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.SetPower(ctx, linear, angular)
}

func (c *reconfigurableClient) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.SetVelocity(ctx, linear, angular)
}

func (c *reconfigurableClient) Stop(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Stop(ctx)
}

func (c *reconfigurableClient) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Do(ctx, cmd)
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

func (c *client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &pb.StopRequest{Name: c.name})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

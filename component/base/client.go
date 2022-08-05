// Package base contains a gRPC based base client
package base

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
	"go.viam.com/rdk/protoutils"
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

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Base {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Base {
	return &client{sc, name}
}

func (c *client) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveStraight(ctx, &pb.MoveStraightRequest{
		Name:       c.name,
		DistanceMm: int64(distanceMm),
		MmPerSec:   mmPerSec,
		Extra:      ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Spin(ctx, &pb.SpinRequest{
		Name:       c.name,
		AngleDeg:   angleDeg,
		DegsPerSec: degsPerSec,
		Extra:      ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.SetPower(ctx, &pb.SetPowerRequest{
		Name:    c.name,
		Linear:  &commonpb.Vector3{X: linear.X, Y: linear.Y, Z: linear.Z},
		Angular: &commonpb.Vector3{X: angular.X, Y: angular.Y, Z: angular.Z},
		Extra:   ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.SetVelocity(ctx, &pb.SetVelocityRequest{
		Name:    c.name,
		Linear:  &commonpb.Vector3{X: linear.X, Y: linear.Y, Z: linear.Z},
		Angular: &commonpb.Vector3{X: angular.X, Y: angular.Y, Z: angular.Z},
		Extra:   ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{Name: c.name, Extra: ext})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

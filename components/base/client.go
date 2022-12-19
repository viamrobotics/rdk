// Package base contains a gRPC based base client
package base

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/base/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
)

// client implements BaseServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.BaseServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Base {
	c := pb.NewBaseServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
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

func (c *client) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
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

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

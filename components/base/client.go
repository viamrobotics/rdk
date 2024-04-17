// Package base contains a gRPC based base client
package base

import (
	"context"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/base/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// client implements BaseServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.BaseServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Base, error) {
	c := pb.NewBaseServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: c,
		logger: logger,
	}, nil
}

// MoveStraight moves the base in a straight line across the given distance at the given velocity.
//
//    myBase, err := base.FromRobot(robot, "my_base")
//
//    // Move the base forward 40 mm at a velocity of 90 mm/s.
//    myBase.MoveStraight(context.Background(), distanceMm: 40, mmPerSec: 90, nil)
//
//    // Move the base backward 40 mm at a velocity of -90 mm/s.
//    myBase.MoveStraight(context.Background(), distanceMm: 40, mmPerSec: -90, nil)
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

// Spin turns the base in place, rotating it to the given angle (degrees) at the given angular velocity (degrees/sec).
//
//    myBase, err := base.FromRobot(robot, "my_base")
//
//    // Spin the base 10 degrees at an angular velocity of 15 deg/sec.
//    myBase.Spin(context.Background(), angleDeg: 10, degsPerSec: 15, nil)
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

// SetPower sets the linear and angular power of the base, represented as a percentage of max power for each direction in the range of [-1.0 to 1.0].
//
//    myBase, err := base.FromRobot(robot, "my_base")
//
//    // Make your wheeled base move forward. Set linear power to 75%.
//    logger.Info("move forward")
//    err = myBase.SetPower(context.Background(), linear: r3.Vector{Y: .75}, angular: r3.Vector{}, nil)
//
//    // Make your wheeled base move backward. Set linear power to -100%.
//    logger.Info("move backward")
//    err = myBase.SetPower(context.Background(), linear: r3.Vector{Y: -1}, angular: r3.Vector{}, nil)
//
//    // Make your wheeled base spin left. Set angular power to 100%.
//    logger.Info("spin left")
//    err = myBase.SetPower(context.Background(), linear: r3.Vector{}, angular: r3.Vector{Z: 1}, nil)
//
//    // Make your wheeled base spin right. Set angular power to -75%.
//    logger.Info("spin right")
//    err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: -.75}, nil)
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
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

func (c *client) Properties(ctx context.Context, extra map[string]interface{}) (Properties, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return Properties{}, err
	}

	req := &pb.GetPropertiesRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetProperties(ctx, req)
	if err != nil {
		return Properties{}, err
	}
	return ProtoFeaturesToProperties(resp), nil
}

func (c *client) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetGeometries(ctx, &commonpb.GetGeometriesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewGeometriesFromProto(resp.GetGeometries())
}

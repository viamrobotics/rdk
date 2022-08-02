// Package imu contains a gRPC based imu client.
package imu

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// serviceClient is a client satisfies the imu.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.IMUServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewIMUServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

var _ = sensor.Sensor(&client{})

// client is an IMU client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) IMU {
	sc := newSvcClientFromConn(conn, logger)
	imu := clientFromSvcClient(sc, name)
	return &reconfigurableIMU{actual: imu}
}

func clientFromSvcClient(sc *serviceClient, name string) IMU {
	return &client{sc, name}
}

func (c *client) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	resp, err := c.client.ReadAngularVelocity(ctx, &pb.ReadAngularVelocityRequest{
		Name: c.name,
	})
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return spatialmath.AngularVelocity{
		X: resp.AngularVelocity.XDegsPerSec,
		Y: resp.AngularVelocity.YDegsPerSec,
		Z: resp.AngularVelocity.ZDegsPerSec,
	}, nil
}

func (c *client) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	resp, err := c.client.ReadOrientation(ctx, &pb.ReadOrientationRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return &spatialmath.EulerAngles{
		Roll:  utils.DegToRad(resp.Orientation.RollDeg),
		Pitch: utils.DegToRad(resp.Orientation.PitchDeg),
		Yaw:   utils.DegToRad(resp.Orientation.YawDeg),
	}, nil
}

func (c *client) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	resp, err := c.client.ReadAcceleration(ctx, &pb.ReadAccelerationRequest{
		Name: c.name,
	})
	if err != nil {
		return r3.Vector{}, err
	}
	return r3.Vector{
		X: resp.Acceleration.XMmPerSecPerSec,
		Y: resp.Acceleration.YMmPerSecPerSec,
		Z: resp.Acceleration.ZMmPerSecPerSec,
	}, nil
}

func (c *client) ReadMagnetometer(ctx context.Context) (r3.Vector, error) {
	resp, err := c.client.ReadMagnetometer(ctx, &pb.ReadMagnetometerRequest{
		Name: c.name,
	})
	if err != nil {
		return r3.Vector{}, err
	}
	return r3.Vector{
		X: resp.Magnetometer.XGauss,
		Y: resp.Magnetometer.YGauss,
		Z: resp.Magnetometer.ZGauss,
	}, nil
}

func (c *client) GetReadings(ctx context.Context) ([]interface{}, error) {
	return GetReadings(ctx, c)
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

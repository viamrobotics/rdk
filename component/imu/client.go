// Package imu contains a gRPC based imu client.
package imu

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/core/grpc"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/spatialmath"
	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"
)

// serviceClient is a client satisfies the imu.proto contract.
type serviceClient struct {
	conn   dialer.ClientConn
	client pb.IMUServiceClient
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
	client := pb.NewIMUServiceClient(conn)
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

// client is an arm client
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, opts rpcclient.DialOptions, logger golog.Logger) (IMU, error) {
	sc, err := newServiceClient(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(conn dialer.ClientConn, name string, logger golog.Logger) IMU {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) IMU {
	return &client{sc, name}
}

// func (ic *imuClient) Readings(ctx context.Context) ([]interface{}, error) {
// 	vel, err := c.AngularVelocity(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	orientation, err := c.Orientation(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	ea := orientation.EulerAngles()
// 	return []interface{}{vel.X, vel.Y, vel.Z, ea.Roll, ea.Pitch, ea.Yaw}, nil
// }

func (c *client) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	resp, err := c.rc.client.IMUAngularVelocity(ctx, &pb.IMUAngularVelocityRequest{
		Name: c.name,
	})
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return spatialmath.AngularVelocity{
		X: resp.AngularVelocity.X,
		Y: resp.AngularVelocity.Y,
		Z: resp.AngularVelocity.Z,
	}, nil
}

func (c *client) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	resp, err := c.rc.client.IMUOrientation(ctx, &pb.IMUOrientationRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return &spatialmath.EulerAngles{
		Roll:  resp.Orientation.Roll,
		Pitch: resp.Orientation.Pitch,
		Yaw:   resp.Orientation.Yaw,
	}, nil
}

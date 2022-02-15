// Package objectmanipulation contains a gRPC based object manipulation client
package objectmanipulation

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/v1"
)

// client is a client satisfies the object_manipulation.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.ObjectManipulationServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewObjectManipulationServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.conn.Close()
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Service, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) DoGrab(ctx context.Context, gripperName, rootName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
	resp, err := c.client.DoGrab(ctx, &pb.ObjectManipulationServiceDoGrabRequest{
		GripperName: gripperName,
		CameraName:  cameraName,
		CameraPoint: vectorToProto(cameraPoint),
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func vectorToProto(v *r3.Vector) *commonpb.Vector3 {
	return &commonpb.Vector3{
		X: v.X,
		Y: v.Y,
		Z: v.Z,
	}
}

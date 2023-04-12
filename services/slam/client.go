// Package slam implements simultaneous localization and mapping
// This is an Experimental package
package slam

import (
	"context"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/utils/rpc"

	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/services/slam/grpchelper"
	"go.viam.com/rdk/spatialmath"
)

// client implements SLAMServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.SLAMServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewSLAMServiceClient(conn)
	c := &client{
		name:   name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c
}

// GetPosition creates a request, calls the slam service GetPosition, and parses the response into a Pose with a component reference string.
func (c *client) GetPosition(ctx context.Context) (spatialmath.Pose, string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetPosition")
	defer span.End()

	req := &pb.GetPositionRequest{
		Name: c.name,
	}

	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return nil, "", err
	}

	p := resp.GetPose()
	componentReference := resp.GetComponentReference()

	return spatialmath.NewPoseFromProtobuf(p), componentReference, nil
}

// GetPointCloudMap creates a request, calls the slam service GetPointCloudMap and returns a callback
// function which will return the next chunk of the current pointcloud map when called.
func (c *client) GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetPointCloudMap")
	defer span.End()

	return grpchelper.GetPointCloudMapCallback(ctx, c.name, c.client)
}

// GetInternalState creates a request, calls the slam service GetInternalState and returns a callback
// function which will return the next chunk of the current internal state of the slam algo when called.
func (c *client) GetInternalState(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetInternalState")
	defer span.End()

	return grpchelper.GetInternalStateCallback(ctx, c.name, c.client)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

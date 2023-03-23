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
func (c *client) GetPosition(ctx context.Context, name string) (spatialmath.Pose, string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetPosition")
	defer span.End()

	req := &pb.GetPositionNewRequest{
		Name: name,
	}

	resp, err := c.client.GetPositionNew(ctx, req)
	if err != nil {
		return nil, "", err
	}

	p := resp.GetPose()
	componentReference := resp.GetComponentReference()

	return spatialmath.NewPoseFromProtobuf(p), componentReference, nil
}

// GetPointCloudMapStream creates a request, calls the slam service GetPointCloudMapStream and returns a callback
// function which will return the next chunk of the current pointcloud map when called.
func (c *client) GetPointCloudMapStream(ctx context.Context, name string) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetPointCloudMapStream")
	defer span.End()

	return grpchelper.GetPointCloudMapStreamCallback(ctx, name, c.client)
}

// GetInternalStateStream creates a request, calls the slam service GetInternalStateStream and returns a callback
// function which will return the next chunk of the current internal state of the slam algo when called.
func (c *client) GetInternalStateStream(ctx context.Context, name string) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetInternalStateStream")
	defer span.End()

	return grpchelper.GetInternalStateStreamCallback(ctx, name, c.client)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

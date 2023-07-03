package slam

import (
	"context"
	"errors"
	"time"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/utils/rpc"

	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam/grpchelper"
	"go.viam.com/rdk/spatialmath"
)

// client implements SLAMServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.SLAMServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger golog.Logger,
) (Service, error) {
	grpcClient := pb.NewSLAMServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
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

// GetLatestMapInfo creates a request, calls the slam service GetLatestMapInfo, and
// returns the timestamp of the last update to the map.
func (c *client) GetLatestMapInfo(ctx context.Context) (time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetLatestMapInfo")
	defer span.End()

	req := &pb.GetLatestMapInfoRequest{
		Name: c.name,
	}

	resp, err := c.client.GetLatestMapInfo(ctx, req)
	if resp == nil { // catch nil return from API (since time default is set to be time.Time{}.UTC())
		return time.Time{}.UTC(), errors.New("failure to get latest map info")
	}
	LastMapUpdate := resp.LastMapUpdate.AsTime()

	return LastMapUpdate, err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

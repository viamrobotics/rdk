package slam

import (
	"context"
	"errors"
	"time"

	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
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
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
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

// Position creates a request, calls the slam service Position, and parses the response into a Pose with a component reference string.
func (c *client) Position(ctx context.Context) (spatialmath.Pose, string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::Position")
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

// PointCloudMap creates a request, calls the slam service PointCloudMap and returns a callback
// function which will return the next chunk of the current pointcloud map when called.
func (c *client) PointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::PointCloudMap")
	defer span.End()

	return grpchelper.PointCloudMapCallback(ctx, c.name, c.client)
}

// InternalState creates a request, calls the slam service InternalState and returns a callback
// function which will return the next chunk of the current internal state of the slam algo when called.
func (c *client) InternalState(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::InternalState")
	defer span.End()

	return grpchelper.InternalStateCallback(ctx, c.name, c.client)
}

// LatestMapInfo creates a request, calls the slam service LatestMapInfo, and
// returns the timestamp of the last update to the map.
func (c *client) LatestMapInfo(ctx context.Context) (time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::LatestMapInfo")
	defer span.End()

	req := &pb.GetLatestMapInfoRequest{
		Name: c.name,
	}

	resp, err := c.client.GetLatestMapInfo(ctx, req)
	if err != nil {
		return time.Time{}, errors.New("failure to get latest map info")
	}
	lastMapUpdate := resp.LastMapUpdate.AsTime()
	return lastMapUpdate, err
}

// LatestMapInfo creates a request, calls the slam service LatestMapInfo, and
// returns the timestamp of the last update to the map.
func (c *client) Properties(ctx context.Context) (Properties, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetProperties")
	defer span.End()

	req := &pb.GetPropertiesRequest{
		Name: c.name,
	}

	resp, err := c.client.GetProperties(ctx, req)
	if err != nil {
		return Properties{}, errors.New("failure to get properties")
	}

	var mappingMode MappingMode
	switch resp.MappingMode {
	case pb.MappingMode_MAPPING_MODE_CREATE_NEW_MAP:
		mappingMode = MappingModeNewMap
	case pb.MappingMode_MAPPING_MODE_LOCALIZE_ONLY:
		mappingMode = MappingModeLocalizationOnly
	case pb.MappingMode_MAPPING_MODE_UPDATE_EXISTING_MAP:
		mappingMode = MappingModeUpdateExistingMap
	case pb.MappingMode_MAPPING_MODE_UNSPECIFIED:
		fallthrough
	default:
		return Properties{}, errors.New("properties error")
	}

	prop := Properties{
		CloudSlam:   resp.CloudSlam,
		MappingMode: mappingMode,
	}
	return prop, err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

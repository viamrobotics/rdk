package slam

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
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
func (c *client) Position(ctx context.Context) (spatialmath.Pose, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::Position")
	defer span.End()

	req := &pb.GetPositionRequest{
		Name: c.name,
	}

	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return nil, err
	}

	p := resp.GetPose()

	return spatialmath.NewPoseFromProtobuf(p), nil
}

// PointCloudMap creates a request, calls the slam service PointCloudMap and returns a callback
// function which will return the next chunk of the current pointcloud map when called.
func (c *client) PointCloudMap(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::PointCloudMap")
	defer span.End()

	return PointCloudMapCallback(ctx, c.name, c.client, returnEditedMap)
}

// InternalState creates a request, calls the slam service InternalState and returns a callback
// function which will return the next chunk of the current internal state of the slam algo when called.
func (c *client) InternalState(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::InternalState")
	defer span.End()

	return InternalStateCallback(ctx, c.name, c.client)
}

// Properties returns information regarding the current SLAM session, including
// if the session is running in the cloud and what mapping mode it is in.
func (c *client) Properties(ctx context.Context) (Properties, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetProperties")
	defer span.End()

	req := &pb.GetPropertiesRequest{
		Name: c.name,
	}

	resp, err := c.client.GetProperties(ctx, req)
	if err != nil {
		return Properties{}, errors.Wrapf(err, "failure to get properties")
	}

	mappingMode, err := protobufToMappingMode(resp.MappingMode)
	if err != nil {
		return Properties{}, err
	}

	sensorInfo := []SensorInfo{}
	for _, sInfo := range resp.SensorInfo {
		sensorType, err := protobufToSensorType(sInfo.Type)
		if err != nil {
			return Properties{}, err
		}
		sensorInfo = append(sensorInfo, SensorInfo{
			Name: sInfo.Name,
			Type: sensorType,
		})
	}

	var internalStateFileType string
	if resp.InternalStateFileType != nil {
		internalStateFileType = *resp.InternalStateFileType
	}

	prop := Properties{
		CloudSlam:             resp.CloudSlam,
		MappingMode:           mappingMode,
		InternalStateFileType: internalStateFileType,
		SensorInfo:            sensorInfo,
	}
	return prop, err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

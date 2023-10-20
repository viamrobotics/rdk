package movementsensor

import (
	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/component/movementsensor/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// client implements MovementSensorServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.MovementSensorServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.ZapCompatibleLogger,
) (MovementSensor, error) {
	c := pb.NewMovementSensorServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: c,
		logger: logging.FromZapCompatible(logger),
	}, nil
}

func (c *client) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.client.GetPosition(ctx, &pb.GetPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, 0, err
	}
	return geo.NewPoint(resp.Coordinate.Latitude, resp.Coordinate.Longitude),
		float64(resp.AltitudeM),
		nil
}

func (c *client) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return r3.Vector{}, err
	}
	resp, err := c.client.GetLinearVelocity(ctx, &pb.GetLinearVelocityRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return r3.Vector{}, err
	}
	return protoutils.ConvertVectorProtoToR3(resp.LinearVelocity), nil
}

func (c *client) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	resp, err := c.client.GetAngularVelocity(ctx, &pb.GetAngularVelocityRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return spatialmath.AngularVelocity(protoutils.ConvertVectorProtoToR3(resp.AngularVelocity)), nil
}

func (c *client) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return r3.Vector{}, err
	}
	resp, err := c.client.GetLinearAcceleration(ctx, &pb.GetLinearAccelerationRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return r3.Vector{}, err
	}
	return protoutils.ConvertVectorProtoToR3(resp.LinearAcceleration), nil
}

func (c *client) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return 0, err
	}
	resp, err := c.client.GetCompassHeading(ctx, &pb.GetCompassHeadingRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return 0, err
	}
	return resp.Value, nil
}

func (c *client) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return spatialmath.NewZeroOrientation(), err
	}
	resp, err := c.client.GetOrientation(ctx, &pb.GetOrientationRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return spatialmath.NewZeroOrientation(), err
	}
	return protoutils.ConvertProtoToOrientation(resp.Orientation), nil
}

func (c *client) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return Readings(ctx, c, extra)
}

func (c *client) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetAccuracy(ctx, &pb.GetAccuracyRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Accuracy, nil
}

func (c *client) Properties(ctx context.Context, extra map[string]interface{}) (*Properties, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return ProtoFeaturesToProperties(resp), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

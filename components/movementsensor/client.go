package movementsensor

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/component/movementsensor/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/spatialmath"
)

// check client fulfills sensor.Sensor interface.
var _ = sensor.Sensor(&client{})

// client implements MovementSensorServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.MovementSensorServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) MovementSensor {
	c := pb.NewMovementSensorServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
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
		float64(resp.AltitudeMm),
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

func (c *client) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	// TODO(erh): should this go over the network?
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
	return resp.AccuracyMm, nil
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
	return (*Properties)(resp), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

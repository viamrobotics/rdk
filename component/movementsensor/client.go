package movementsensor

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/movementsensor/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/spatialmath"
)

// serviceClient is a client satisfies the gps.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.MovementSensorServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewMovementSensorServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

var _ = sensor.Sensor(&client{})

// client is a MovementSensor client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) MovementSensor {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) MovementSensor {
	return &client{sc, name}
}

func (c *client) GetPosition(ctx context.Context) (*geo.Point, float64, float64, error) {
	resp, err := c.client.GetPosition(ctx, &pb.GetPositionRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, 0, 0, err
	}
	return geo.NewPoint(resp.Coordinate.Latitude, resp.Coordinate.Longitude), float64(resp.AltitudeMm), float64(resp.AccuracyMm), nil
}

func (c *client) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	resp, err := c.client.GetLinearVelocity(ctx, &pb.GetLinearVelocityRequest{
		Name: c.name,
	})
	if err != nil {
		return r3.Vector{}, err
	}
	return protoutils.ConvertVectorProtoToR3(resp.LinearVelocity), nil
}

func (c *client) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	resp, err := c.client.GetAngularVelocity(ctx, &pb.GetAngularVelocityRequest{
		Name: c.name,
	})
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return spatialmath.AngularVelocity(protoutils.ConvertVectorProtoToR3(resp.AngularVelocity)), nil
}

func (c *client) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	resp, err := c.client.GetOrientation(ctx, &pb.GetOrientationRequest{
		Name: c.name,
	})
	if err != nil {
		return spatialmath.NewZeroOrientation(), err
	}
	return protoutils.ConvertProtoToOrientation(resp.Orientation), nil
}

func (c *client) GetCompassHeading(ctx context.Context) (float64, error) {
	resp, err := c.client.GetCompassHeading(ctx, &pb.GetCompassHeadingRequest{
		Name: c.name,
	})
	if err != nil {
		return 0, err
	}
	return resp.Value, nil
}

func (c *client) GetReadings(ctx context.Context) ([]interface{}, error) {
	// TODO(erh): should this go over the network?
	return GetReadings(ctx, c)
}

func (c *client) GetProperties(ctx context.Context) (*Properties, error) {
	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return (*Properties)(resp), nil
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

// Package gps contains a gRPC based gps client.
package gps

import (
	"context"
	"math"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/gps/v1"
)

// serviceClient is a client satisfies the gps.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.GPSServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewGPSServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return nil
}

var _ = sensor.Sensor(&client{})

// client is a GPS client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) GPS {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) GPS {
	return &client{sc, name}
}

func (c *client) ReadLocation(ctx context.Context) (*geo.Point, error) {
	resp, err := c.client.ReadLocation(ctx, &pb.ReadLocationRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return geo.NewPoint(resp.Coordinate.Latitude, resp.Coordinate.Longitude), nil
}

func (c *client) ReadAltitude(ctx context.Context) (float64, error) {
	resp, err := c.client.ReadAltitude(ctx, &pb.ReadAltitudeRequest{
		Name: c.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.AltitudeMeters, nil
}

func (c *client) ReadSpeed(ctx context.Context) (float64, error) {
	resp, err := c.client.ReadSpeed(ctx, &pb.ReadSpeedRequest{
		Name: c.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.SpeedMmPerSec, nil
}

func (c *client) GetReadings(ctx context.Context) ([]interface{}, error) {
	return GetReadings(ctx, c)
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

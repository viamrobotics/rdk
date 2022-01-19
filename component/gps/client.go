// Package gps contains a gRPC based gps client.
package gps

import (
	"context"
	"math"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// serviceClient is a client satisfies the gps.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.GPSServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
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
	return sc.conn.Close()
}

// client is a GPS client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (GPS, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) GPS {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) GPS {
	return &client{sc, name}
}

func (c *client) Location(ctx context.Context) (*geo.Point, error) {
	resp, err := c.client.Location(ctx, &pb.GPSServiceLocationRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return geo.NewPoint(resp.Coordinate.Latitude, resp.Coordinate.Longitude), nil
}

func (c *client) Altitude(ctx context.Context) (float64, error) {
	resp, err := c.client.Altitude(ctx, &pb.GPSServiceAltitudeRequest{
		Name: c.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.Altitude, nil
}

func (c *client) Speed(ctx context.Context) (float64, error) {
	resp, err := c.client.Speed(ctx, &pb.GPSServiceSpeedRequest{
		Name: c.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.SpeedKph, nil
}

func (c *client) Accuracy(ctx context.Context) (float64, float64, error) {
	resp, err := c.client.Accuracy(ctx, &pb.GPSServiceAccuracyRequest{
		Name: c.name,
	})
	if err != nil {
		return math.NaN(), math.NaN(), err
	}
	return resp.HorizontalAccuracy, resp.VerticalAccuracy, nil
}

func (c *client) Readings(ctx context.Context) ([]interface{}, error) {
	loc, err := c.Location(ctx)
	if err != nil {
		return nil, err
	}
	alt, err := c.Altitude(ctx)
	if err != nil {
		return nil, err
	}
	speed, err := c.Speed(ctx)
	if err != nil {
		return nil, err
	}
	horzAcc, vertAcc, err := c.Accuracy(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{loc.Lat(), loc.Lng(), alt, speed, horzAcc, vertAcc}, nil
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

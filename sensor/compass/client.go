package compass

import (
	"context"
	"math"

	pb "go.viam.com/robotcore/proto/sensor/compass/v1"

	"google.golang.org/grpc"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.CompassServiceClient
}

func NewClient(ctx context.Context, address string) (Device, error) {
	// TODO(erd): address insecure
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	client := pb.NewCompassServiceClient(conn)
	return &Client{conn, client}, nil
}

func (c *Client) StartCalibration(ctx context.Context) error {
	_, err := c.client.StartCalibration(ctx, &pb.StartCalibrationRequest{})
	return err
}

func (c *Client) StopCalibration(ctx context.Context) error {
	_, err := c.client.StopCalibration(ctx, &pb.StopCalibrationRequest{})
	return err
}

func (c *Client) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := c.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (c *Client) Heading(ctx context.Context) (float64, error) {
	resp, err := c.client.Heading(ctx, &pb.HeadingRequest{})
	if err != nil {
		return math.NaN(), err
	}
	return resp.Heading, nil
}

func (c *Client) Close(ctx context.Context) error {
	return c.conn.Close()
}

package lidar

import (
	"context"
	"image"
	"math"

	"go.uber.org/multierr"
	pb "go.viam.com/robotcore/proto/lidar/v1"

	"google.golang.org/grpc"
)

const ModelNameClient = "lidarclient"
const DeviceTypeClient = DeviceType("lidarclient")

func init() {
	RegisterDeviceType(DeviceTypeClient, DeviceTypeRegistration{
		New: func(ctx context.Context, desc DeviceDescription) (Device, error) {
			return NewClient(ctx, desc.Path)
		},
	})
}

type Client struct {
	conn   *grpc.ClientConn
	client pb.LidarServiceClient
}

func NewClient(ctx context.Context, address string) (Device, error) {
	// TODO(erd): address insecure
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	client := pb.NewLidarServiceClient(conn)
	return &Client{conn, client}, nil
}

func (c *Client) Info(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.Info(ctx, &pb.InfoRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Info.AsMap(), nil
}

func (c *Client) Start(ctx context.Context) error {
	_, err := c.client.Start(ctx, &pb.StartRequest{})
	return err
}

func (c *Client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &pb.StopRequest{})
	return err
}

func (c *Client) Close(ctx context.Context) (err error) {
	defer func() {
		err = multierr.Combine(err, c.conn.Close())
	}()
	return c.Stop(ctx)
}

func (c *Client) Scan(ctx context.Context, options ScanOptions) (Measurements, error) {
	resp, err := c.client.Scan(ctx, &pb.ScanRequest{})
	if err != nil {
		return nil, err
	}
	return MeasurementsFromProto(resp.Measurements), nil
}

func (c *Client) Range(ctx context.Context) (int, error) {
	resp, err := c.client.Range(ctx, &pb.RangeRequest{})
	if err != nil {
		return 0, err
	}
	return int(resp.Range), nil
}

func (c *Client) Bounds(ctx context.Context) (image.Point, error) {
	resp, err := c.client.Bounds(ctx, &pb.BoundsRequest{})
	if err != nil {
		return image.Point{}, err
	}
	return image.Point{int(resp.X), int(resp.Y)}, nil
}

func (c *Client) AngularResolution(ctx context.Context) (float64, error) {
	resp, err := c.client.AngularResolution(ctx, &pb.AngularResolutionRequest{})
	if err != nil {
		return math.NaN(), err
	}
	return resp.AngularResolution, nil
}

func ScanOptionsFromProto(req *pb.ScanRequest) ScanOptions {
	return ScanOptions{
		Count:    int(req.Count),
		NoFilter: req.NoFilter,
	}
}

func ScanOptionsToProto(opts ScanOptions) *pb.ScanRequest {
	return &pb.ScanRequest{
		Count:    int32(opts.Count),
		NoFilter: opts.NoFilter,
	}
}

func MeasurementFromProto(pm *pb.Measurement) *Measurement {
	return &Measurement{
		data: measurementData{
			Angle:    pm.Angle,
			AngleDeg: pm.AngleDeg,
			Distance: pm.Distance,
			X:        pm.X,
			Y:        pm.Y,
		},
	}
}

func MeasurementsFromProto(pms []*pb.Measurement) Measurements {
	ms := make(Measurements, 0, len(pms))
	for _, pm := range pms {
		ms = append(ms, MeasurementFromProto(pm))
	}
	return ms
}

func MeasurementToProto(m *Measurement) *pb.Measurement {
	return &pb.Measurement{
		Angle:    m.data.Angle,
		AngleDeg: m.data.AngleDeg,
		Distance: m.data.Distance,
		X:        m.data.X,
		Y:        m.data.Y,
	}
}

func MeasurementsToProto(ms Measurements) []*pb.Measurement {
	pms := make([]*pb.Measurement, 0, len(ms))
	for _, m := range ms {
		pms = append(pms, MeasurementToProto(m))
	}
	return pms
}

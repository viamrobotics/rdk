package lidar_test

import (
	"context"
	"errors"
	"image"
	"math"
	"testing"

	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/lidar/v1"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

func TestServer(t *testing.T) {
	device := &inject.LidarDevice{}
	server := lidar.NewServer(device)

	err1 := errors.New("whoops")
	device.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return nil, err1
	}
	_, err := server.Info(context.Background(), &pb.InfoRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"hello": true}, nil
	}
	infoResp, err := server.Info(context.Background(), &pb.InfoRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, infoResp.GetInfo().AsMap(), test.ShouldResemble, map[string]interface{}{"hello": true})

	device.StartFunc = func(ctx context.Context) error {
		return err1
	}
	_, err = server.Start(context.Background(), &pb.StartRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.StartFunc = func(ctx context.Context) error {
		return nil
	}
	_, err = server.Start(context.Background(), &pb.StartRequest{})
	test.That(t, err, test.ShouldBeNil)

	device.StopFunc = func(ctx context.Context) error {
		return err1
	}
	_, err = server.Stop(context.Background(), &pb.StopRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.StopFunc = func(ctx context.Context) error {
		return nil
	}
	_, err = server.Stop(context.Background(), &pb.StopRequest{})
	test.That(t, err, test.ShouldBeNil)

	device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return nil, err1
	}
	_, err = server.Scan(context.Background(), &pb.ScanRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	var capOptions lidar.ScanOptions
	ms := lidar.Measurements{lidar.NewMeasurement(0, 1), lidar.NewMeasurement(1, 2)}
	device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		capOptions = options
		return ms, nil
	}
	scanResp, err := server.Scan(context.Background(), &pb.ScanRequest{Count: 4, NoFilter: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lidar.MeasurementsFromProto(scanResp.GetMeasurements()), test.ShouldResemble, ms)
	test.That(t, capOptions, test.ShouldResemble, lidar.ScanOptions{Count: 4, NoFilter: true})

	device.RangeFunc = func(ctx context.Context) (int, error) {
		return 0, err1
	}
	_, err = server.Range(context.Background(), &pb.RangeRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.RangeFunc = func(ctx context.Context) (int, error) {
		return 5, nil
	}
	rangeResp, err := server.Range(context.Background(), &pb.RangeRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rangeResp.GetRange(), test.ShouldEqual, 5)

	device.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{}, err1
	}
	_, err = server.Bounds(context.Background(), &pb.BoundsRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{4, 5}, nil
	}
	boundsResp, err := server.Bounds(context.Background(), &pb.BoundsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, boundsResp.GetX(), test.ShouldEqual, 4)
	test.That(t, boundsResp.GetY(), test.ShouldEqual, 5)

	device.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), err1
	}
	_, err = server.AngularResolution(context.Background(), &pb.AngularResolutionRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 6.2, nil
	}
	angResp, err := server.AngularResolution(context.Background(), &pb.AngularResolutionRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angResp.GetAngularResolution(), test.ShouldEqual, 6.2)
}

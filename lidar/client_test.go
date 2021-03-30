package lidar_test

import (
	"context"
	"errors"
	"image"
	"math"
	"net"
	"testing"

	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/lidar/v1"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
	"google.golang.org/grpc"
)

func TestClient(t *testing.T) {
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	injectDev1 := &inject.LidarDevice{}
	injectDev2 := &inject.LidarDevice{}
	pb.RegisterLidarServiceServer(gServer1, lidar.NewServer(injectDev1))
	pb.RegisterLidarServiceServer(gServer2, lidar.NewServer(injectDev2))

	injectDev1.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return nil, errors.New("whoops")
	}
	injectDev1.StartFunc = func(ctx context.Context) error {
		return errors.New("whoops")
	}
	injectDev1.StopFunc = func(ctx context.Context) error {
		return errors.New("whoops")
	}
	injectDev1.CloseFunc = func(ctx context.Context) error {
		return errors.New("whoops")
	}
	injectDev1.ScanFunc = func(ctx context.Context, opts lidar.ScanOptions) (lidar.Measurements, error) {
		return nil, errors.New("whoops")
	}
	injectDev1.RangeFunc = func(ctx context.Context) (int, error) {
		return 0, errors.New("whoops")
	}
	injectDev1.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{}, errors.New("whoops")
	}
	injectDev1.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}

	injectDev2.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"hello": "world"}, nil
	}
	injectDev2.StartFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev2.StopFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev2.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev2.ScanFunc = func(ctx context.Context, opts lidar.ScanOptions) (lidar.Measurements, error) {
		return lidar.Measurements{lidar.NewMeasurement(2, 40)}, nil
	}
	injectDev2.RangeFunc = func(ctx context.Context) (int, error) {
		return 25, nil
	}
	injectDev2.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{4, 5}, nil
	}
	injectDev2.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = lidar.NewClient(cancelCtx, listener1.Addr().String())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	client, err := lidar.NewClient(context.Background(), listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	_, err = client.Info(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = client.Start(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = client.Stop(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = client.Scan(context.Background(), lidar.ScanOptions{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = client.Range(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = client.Bounds(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = client.AngularResolution(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = client.Close(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	client, err = lidar.NewClient(context.Background(), listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	info, err := client.Info(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info, test.ShouldResemble, map[string]interface{}{"hello": "world"})
	err = client.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = client.Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	scan, err := client.Scan(context.Background(), lidar.ScanOptions{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, scan, test.ShouldResemble, lidar.Measurements{lidar.NewMeasurement(2, 40)})
	devRange, err := client.Range(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, devRange, test.ShouldEqual, 25)
	bounds, err := client.Bounds(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bounds, test.ShouldResemble, image.Point{4, 5})
	angRes, err := client.AngularResolution(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angRes, test.ShouldEqual, 5.2)
	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

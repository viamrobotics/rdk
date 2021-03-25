package compass_test

import (
	"context"
	"errors"
	"math"
	"net"
	"testing"

	pb "go.viam.com/robotcore/proto/sensor/compass/v1"
	"go.viam.com/robotcore/sensor/compass"
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
	injectDev1 := &inject.Compass{}
	injectDev2 := &inject.Compass{}
	pb.RegisterCompassServiceServer(gServer1, compass.NewServer(injectDev1))
	pb.RegisterCompassServiceServer(gServer2, compass.NewServer(injectDev2))

	injectDev1.HeadingFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}
	injectDev1.StartCalibrationFunc = func(ctx context.Context) error {
		return errors.New("whoops")
	}
	injectDev1.StopCalibrationFunc = func(ctx context.Context) error {
		return errors.New("whoops")
	}

	injectDev2.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 23.45, nil
	}
	injectDev2.StartCalibrationFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev2.StopCalibrationFunc = func(ctx context.Context) error {
		return nil
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = compass.NewClient(cancelCtx, listener1.Addr().String())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	dev, err := compass.NewClient(context.Background(), listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	err = dev.StartCalibration(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = dev.StopCalibration(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.Heading(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.Readings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, dev.Close(context.Background()), test.ShouldBeNil)

	dev, err = compass.NewClient(context.Background(), listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dev.StartCalibration(context.Background()), test.ShouldBeNil)
	test.That(t, dev.StopCalibration(context.Background()), test.ShouldBeNil)
	heading, err := dev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 23.45)
	readings, err := dev.Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, []interface{}{23.45})
	test.That(t, dev.Close(context.Background()), test.ShouldBeNil)
}

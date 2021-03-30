package compass_test

import (
	"context"
	"errors"
	"math"
	"testing"

	pb "go.viam.com/robotcore/proto/sensor/compass/v1"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

func TestServer(t *testing.T) {
	device := &inject.Compass{}
	server := compass.NewServer(device)

	err1 := errors.New("whoops")
	device.HeadingFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), err1
	}
	_, err := server.Heading(context.Background(), &pb.HeadingRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 6.2, nil
	}
	headingResp, err := server.Heading(context.Background(), &pb.HeadingRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingResp.GetHeading(), test.ShouldEqual, 6.2)

	device.StartCalibrationFunc = func(ctx context.Context) error {
		return err1
	}
	_, err = server.StartCalibration(context.Background(), &pb.StartCalibrationRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.StartCalibrationFunc = func(ctx context.Context) error {
		return nil
	}
	_, err = server.StartCalibration(context.Background(), &pb.StartCalibrationRequest{})
	test.That(t, err, test.ShouldBeNil)

	device.StopCalibrationFunc = func(ctx context.Context) error {
		return err1
	}
	_, err = server.StopCalibration(context.Background(), &pb.StopCalibrationRequest{})
	test.That(t, err, test.ShouldEqual, err1)
	device.StopCalibrationFunc = func(ctx context.Context) error {
		return nil
	}
	_, err = server.StopCalibration(context.Background(), &pb.StopCalibrationRequest{})
	test.That(t, err, test.ShouldBeNil)
}

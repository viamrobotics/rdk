package imu_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/component/imu"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils/inject"
)

func newServer() (pb.IMUServiceServer, *inject.IMU, error) {
	injectIMU := &inject.IMU{}
	imus := map[resource.Name]interface{}{
		imu.Named("imu"): injectIMU,
		imu.Named("imu2"): "notIMU",
	}
	imuSvc, err := subtype.New(imus)
	if err != nil {
		return nil, nil, err
	}
	return imu.NewServer(imuSvc), injectIMU, nil
}

func TestServer(t *testing.T) {
	imuServer, injectIMU, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	av := &pb.AngularVelocity{X: 1, Y: 2, Z: 3}
	ea := &pb.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6}
	injectIMU.AngularVelocityFunc = func(ctx context.Context) (*pb.AngularVelocity, error) {
		return av, nil
	}
	injectIMU.OrientationFunc = func(ctx context.Context) (*pb.EulerAngles, error) {
		return ea, nil
	}

	t.Run("IMU angular velocity", func(t *testing.T) {
		resp, err := imuServer.AngularVelocity(context.Background(), &pb.IMUServiceAngularVelocityRequest{Name: "imu1"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AngularVelocity.String(), test.ShouldResemble, av.String())

		_, err = imuServer.AngularVelocity(context.Background(), &pb.IMUServiceAngularVelocityRequest{Name: "imu2"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.AngularVelocity(context.Background(), &pb.IMUServiceAngularVelocityRequest{Name: "imu3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})

	t.Run("IMU orientation", func(t *testing.T) {
		resp, err := imuServer.Orientation(context.Background(), &pb.IMUServiceOrientationRequest{Name: "imu1"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Orientation.String(), test.ShouldResemble, ea.String())

		_, err = imuServer.Orientation(context.Background(), &pb.IMUServiceOrientationRequest{Name: "imu2"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.Orientation(context.Background(), &pb.IMUServiceOrientationRequest{Name: "imu3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})
}

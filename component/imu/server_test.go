package imu_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/imu"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

const (
	testIMUName    = "imu1"
	fakeIMUName    = "imu2"
	missingIMUName = "imu3"
)

func newServer() (pb.IMUServiceServer, *inject.IMU, error) {
	injectIMU := &inject.IMU{}
	imus := map[resource.Name]interface{}{
		imu.Named(testIMUName): injectIMU,
		imu.Named(fakeIMUName): "notIMU",
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

	injectIMU.ReadAngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3}, nil
	}
	injectIMU.ReadOrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
		return &spatialmath.EulerAngles{Roll: utils.DegToRad(4), Pitch: utils.DegToRad(5), Yaw: utils.DegToRad(6)}, nil
	}

	//nolint:dupl
	t.Run("IMU angular velocity", func(t *testing.T) {
		resp, err := imuServer.ReadAngularVelocity(context.Background(), &pb.IMUServiceReadAngularVelocityRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AngularVelocity, test.ShouldResemble, &pb.AngularVelocity{XDegsPerSec: 1, YDegsPerSec: 2, ZDegsPerSec: 3})

		_, err = imuServer.ReadAngularVelocity(context.Background(), &pb.IMUServiceReadAngularVelocityRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.ReadAngularVelocity(context.Background(), &pb.IMUServiceReadAngularVelocityRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})

	//nolint:dupl
	t.Run("IMU orientation", func(t *testing.T) {
		resp, err := imuServer.ReadOrientation(context.Background(), &pb.IMUServiceReadOrientationRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Orientation, test.ShouldResemble, &pb.EulerAngles{RollDeg: 4, PitchDeg: 5, YawDeg: 6})

		_, err = imuServer.ReadOrientation(context.Background(), &pb.IMUServiceReadOrientationRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.ReadOrientation(context.Background(), &pb.IMUServiceReadOrientationRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})
}

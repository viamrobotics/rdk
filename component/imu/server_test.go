package imu_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/imu"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
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

	injectIMU.ReadAccelerationFunc = func(ctx context.Context) (r3.Vector, error) {
		return r3.Vector{X: 7, Y: 8, Z: 9}, nil
	}
	injectIMU.ReadMagnetometerFunc = func(ctx context.Context) (r3.Vector, error) {
		return r3.Vector{X: 10, Y: 11, Z: 12}, nil
	}

	//nolint:dupl
	t.Run("IMU angular velocity", func(t *testing.T) {
		resp, err := imuServer.ReadAngularVelocity(context.Background(), &pb.ReadAngularVelocityRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AngularVelocity, test.ShouldResemble, &pb.AngularVelocity{XDegsPerSec: 1, YDegsPerSec: 2, ZDegsPerSec: 3})

		_, err = imuServer.ReadAngularVelocity(context.Background(), &pb.ReadAngularVelocityRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.ReadAngularVelocity(context.Background(), &pb.ReadAngularVelocityRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})

	//nolint:dupl
	t.Run("IMU orientation", func(t *testing.T) {
		resp, err := imuServer.ReadOrientation(context.Background(), &pb.ReadOrientationRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Orientation, test.ShouldResemble, &pb.EulerAngles{RollDeg: 4, PitchDeg: 5, YawDeg: 6})

		_, err = imuServer.ReadOrientation(context.Background(), &pb.ReadOrientationRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.ReadOrientation(context.Background(), &pb.ReadOrientationRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})

	//nolint:dupl
	t.Run("IMU acceleration", func(t *testing.T) {
		resp, err := imuServer.ReadAcceleration(context.Background(), &pb.ReadAccelerationRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Acceleration, test.ShouldResemble, &pb.Acceleration{XMmPerSecPerSec: 7, YMmPerSecPerSec: 8, ZMmPerSecPerSec: 9})

		_, err = imuServer.ReadAcceleration(context.Background(), &pb.ReadAccelerationRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.ReadAcceleration(context.Background(), &pb.ReadAccelerationRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})

	//nolint:dupl
	t.Run("IMU magnetometer", func(t *testing.T) {
		resp, err := imuServer.ReadMagnetometer(context.Background(), &pb.ReadMagnetometerRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Magnetometer, test.ShouldResemble, &pb.Magnetometer{XGauss: 10, YGauss: 11, ZGauss: 12})

		_, err = imuServer.ReadMagnetometer(context.Background(), &pb.ReadMagnetometerRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.ReadMagnetometer(context.Background(), &pb.ReadMagnetometerRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})
}

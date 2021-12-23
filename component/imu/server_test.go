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
)

const testIMUName = "imu1"
const fakeIMUName = "imu2"
const missingIMUName = "imu3"

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

	injectIMU.AngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {

		return spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3}, nil
	}
	injectIMU.OrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
		return &spatialmath.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6}, nil
	}

	t.Run("IMU angular velocity", func(t *testing.T) {
		resp, err := imuServer.AngularVelocity(context.Background(), &pb.IMUServiceAngularVelocityRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AngularVelocity, test.ShouldResemble, &pb.AngularVelocity{X: 1, Y: 2, Z: 3})

		_, err = imuServer.AngularVelocity(context.Background(), &pb.IMUServiceAngularVelocityRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.AngularVelocity(context.Background(), &pb.IMUServiceAngularVelocityRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})

	t.Run("IMU orientation", func(t *testing.T) {
		resp, err := imuServer.Orientation(context.Background(), &pb.IMUServiceOrientationRequest{Name: testIMUName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Orientation, test.ShouldResemble, &pb.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6})

		_, err = imuServer.Orientation(context.Background(), &pb.IMUServiceOrientationRequest{Name: fakeIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an IMU")

		_, err = imuServer.Orientation(context.Background(), &pb.IMUServiceOrientationRequest{Name: missingIMUName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no IMU")
	})
}

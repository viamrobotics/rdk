package imu_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/imu"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	imu1 := "imu1"

	av := spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3}
	ea := &spatialmath.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6}
	rs := []interface{}{av.X, av.Y, av.Z, ea.Roll, ea.Pitch, ea.Yaw}
	desc := sensor.Description{sensor.Type("imu"), ""}

	injectIMU := &inject.IMU{}
	injectIMU.AngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
		return av, nil
	}
	injectIMU.OrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
		return ea, nil
	}
	injectIMU.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
		return rs, nil
	}
	injectIMU.DescFunc = func() sensor.Description {
		return desc
	}

	imuSvc, err := subtype.New((map[resource.Name]interface{}{imu.Named(imu1): injectIMU}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterIMUServiceServer(gServer, imu.NewServer(imuSvc))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = imu.NewClient(cancelCtx, imu1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("IMU client 1", func(t *testing.T) {
		// working
		imu1Client, err := imu.NewClient(context.Background(), imu1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		av1, err := imu1Client.AngularVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av1, test.ShouldResemble, av)

		ea1, err := imu1Client.Orientation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ea1, test.ShouldResemble, ea)

		rs1, err := imu1Client.Readings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		desc1 := imu1Client.Desc()
		test.That(t, desc1, test.ShouldResemble, desc)
		test.That(t, utils.TryClose(context.Background(), imu1Client), test.ShouldBeNil)
	})

	t.Run("IMU client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		imu1Client2 := imu.NewClientFromConn(context.Background(), conn, imu1, logger)

		av2, err := imu1Client2.AngularVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av2, test.ShouldResemble, av)

		ea2, err := imu1Client2.Orientation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ea2, test.ShouldResemble, ea)

		rs2, err := imu1Client2.Readings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs2, test.ShouldResemble, rs)

		desc2 := imu1Client2.Desc()
		test.That(t, desc2, test.ShouldResemble, desc)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientZeroValues(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	imu1 := "imu1"

	av := spatialmath.AngularVelocity{X: 0, Y: 0, Z: 0}
	ea := &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: 0}
	rs := []interface{}{av.X, av.Y, av.Z, ea.Roll, ea.Pitch, ea.Yaw}

	injectIMU := &inject.IMU{}
	injectIMU.AngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
		return av, nil
	}
	injectIMU.OrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
		return ea, nil
	}
	injectIMU.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
		return rs, nil
	}

	imuSvc, err := subtype.New((map[resource.Name]interface{}{imu.Named(imu1): injectIMU}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterIMUServiceServer(gServer, imu.NewServer(imuSvc))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	t.Run("IMU client", func(t *testing.T) {
		imu1Client, err := imu.NewClient(context.Background(), imu1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		av1, err := imu1Client.AngularVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av1, test.ShouldResemble, av)

		ea1, err := imu1Client.Orientation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ea1, test.ShouldResemble, ea)

		rs1, err := imu1Client.Readings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		test.That(t, utils.TryClose(context.Background(), imu1Client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectIMU := &inject.IMU{}
	imu1 := "imu1"

	imuSvc, err := subtype.New((map[resource.Name]interface{}{imu.Named(imu1): injectIMU}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterIMUServiceServer(gServer, imu.NewServer(imuSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := imu.NewClient(ctx, imu1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := imu.NewClient(ctx, imu1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

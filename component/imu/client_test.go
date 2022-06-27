package imu_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	av := spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3}
	ea := &spatialmath.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6}
	ac := r3.Vector{X: 7, Y: 8, Z: 9}
	mg := r3.Vector{X: 10, Y: 11, Z: 12}
	rs := []interface{}{
		av.X, av.Y, av.Z,
		ea.Roll, ea.Pitch, ea.Yaw,
		ac.X, ac.Y, ac.Z,
		mg.X, mg.Y, mg.Z,
	}

	injectIMU := &inject.IMU{}
	injectIMU.ReadAngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
		return av, nil
	}
	injectIMU.ReadOrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
		return ea, nil
	}
	injectIMU.ReadAccelerationFunc = func(ctx context.Context) (r3.Vector, error) {
		return ac, nil
	}
	injectIMU.ReadMagnetometerFunc = func(ctx context.Context) (r3.Vector, error) {
		return mg, nil
	}

	imuSvc, err := subtype.New(map[resource.Name]interface{}{imu.Named(testIMUName): injectIMU})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(imu.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, imuSvc)

	injectIMU.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, imuSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("IMU client 1", func(t *testing.T) {
		// working
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		imu1Client := imu.NewClientFromConn(context.Background(), conn, testIMUName, logger)

		// Do
		resp, err := imu1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		av1, err := imu1Client.ReadAngularVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av1, test.ShouldResemble, av)

		ea1, err := imu1Client.ReadOrientation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ea1, test.ShouldResemble, ea)

		ac1, err := imu1Client.ReadAcceleration(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ac1, test.ShouldResemble, ac)

		mg1, err := imu1Client.ReadMagnetometer(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mg1, test.ShouldResemble, mg)

		rs1, err := imu1Client.(sensor.Sensor).GetReadings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		test.That(t, utils.TryClose(context.Background(), imu1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("IMU client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testIMUName, logger)
		imu1Client2, ok := client.(imu.IMU)
		test.That(t, ok, test.ShouldBeTrue)

		av2, err := imu1Client2.ReadAngularVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av2, test.ShouldResemble, av)

		ea2, err := imu1Client2.ReadOrientation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ea2, test.ShouldResemble, ea)

		ac2, err := imu1Client2.ReadAcceleration(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ac2, test.ShouldResemble, ac)

		mg2, err := imu1Client2.ReadMagnetometer(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mg2, test.ShouldResemble, mg)

		rs2, err := imu1Client2.(sensor.Sensor).GetReadings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs2, test.ShouldResemble, rs)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientZeroValues(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	av := spatialmath.AngularVelocity{X: 0, Y: 0, Z: 0}
	ea := &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: 0}
	ac := r3.Vector{X: 0, Y: 0, Z: 0}
	mg := r3.Vector{X: 0, Y: 0, Z: 0}
	rs := []interface{}{
		av.X, av.Y, av.Z,
		ea.Roll, ea.Pitch, ea.Yaw,
		ac.X, ac.Y, ac.Z,
		mg.X, mg.Y, mg.Z,
	}

	injectIMU := &inject.IMU{}
	injectIMU.ReadAngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
		return av, nil
	}
	injectIMU.ReadOrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
		return ea, nil
	}
	injectIMU.ReadAccelerationFunc = func(ctx context.Context) (r3.Vector, error) {
		return ac, nil
	}
	injectIMU.ReadMagnetometerFunc = func(ctx context.Context) (r3.Vector, error) {
		return mg, nil
	}

	imuSvc, err := subtype.New(map[resource.Name]interface{}{imu.Named(testIMUName): injectIMU})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterIMUServiceServer(gServer, imu.NewServer(imuSvc))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	t.Run("IMU client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		imu1Client := imu.NewClientFromConn(context.Background(), conn, testIMUName, logger)
		test.That(t, err, test.ShouldBeNil)

		av1, err := imu1Client.ReadAngularVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av1, test.ShouldResemble, av)

		ea1, err := imu1Client.ReadOrientation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ea1, test.ShouldResemble, ea)

		ac1, err := imu1Client.ReadAcceleration(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ac1, test.ShouldResemble, ac)

		mg1, err := imu1Client.ReadMagnetometer(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mg1, test.ShouldResemble, mg)

		rs1, err := imu1Client.(sensor.Sensor).GetReadings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		test.That(t, utils.TryClose(context.Background(), imu1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectIMU := &inject.IMU{}

	imuSvc, err := subtype.New(map[resource.Name]interface{}{imu.Named(testIMUName): injectIMU})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterIMUServiceServer(gServer, imu.NewServer(imuSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := imu.NewClientFromConn(ctx, conn1, testIMUName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := imu.NewClientFromConn(ctx, conn2, testIMUName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

package motor_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/motor/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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

	workingMotor := &inject.Motor{}
	failingMotor := &inject.Motor{}

	var actualExtra map[string]interface{}

	workingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		actualExtra = extra
		return 42.0, nil
	}
	workingMotor.GetFeaturesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
		actualExtra = extra
		return map[motor.Feature]bool{
			motor.PositionReporting: true,
		}, nil
	}
	workingMotor.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		actualExtra = extra
		return true, nil
	}

	failingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		return errors.New("set power failed")
	}
	failingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return errors.New("go for failed")
	}
	failingMotor.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		return errors.New("go to failed")
	}
	failingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		return errors.New("set to zero failed")
	}
	failingMotor.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, errors.New("position unavailable")
	}
	failingMotor.GetFeaturesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
		return nil, errors.New("supported features unavailable")
	}
	failingMotor.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("stop failed")
	}
	failingMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return false, errors.New("is on unavailable")
	}

	resourceMap := map[resource.Name]interface{}{
		motor.Named(testMotorName): workingMotor,
		motor.Named(failMotorName): failingMotor,
	}
	motorSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(motor.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, motorSvc)

	workingMotor.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, motorSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingMotorClient := motor.NewClientFromConn(context.Background(), conn, testMotorName, logger)

	t.Run("client tests for working motor", func(t *testing.T) {
		// Do
		resp, err := workingMotorClient.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		err = workingMotorClient.SetPower(context.Background(), 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.GoFor(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.GoTo(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.ResetZeroPosition(context.Background(), 0.5, nil)
		test.That(t, err, test.ShouldBeNil)

		pos, err := workingMotorClient.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		features, err := workingMotorClient.GetFeatures(context.Background(), nil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		isOn, err := workingMotorClient.IsPowered(context.Background(), map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})

		test.That(t, utils.TryClose(context.Background(), workingMotorClient), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	conn, err = viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	failingMotorClient := motor.NewClientFromConn(context.Background(), conn, failMotorName, logger)

	t.Run("client tests for failing motor", func(t *testing.T) {
		err := failingMotorClient.GoTo(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.ResetZeroPosition(context.Background(), 0.5, nil)
		test.That(t, err, test.ShouldNotBeNil)

		pos, err := failingMotorClient.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		err = failingMotorClient.SetPower(context.Background(), 42.0, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.GoFor(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldNotBeNil)

		features, err := failingMotorClient.GetFeatures(context.Background(), nil)
		test.That(t, features, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		isOn, err := failingMotorClient.IsPowered(context.Background(), nil)
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)

		test.That(t, utils.TryClose(context.Background(), failingMotorClient), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working motor", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingMotorDialedClient := motor.NewClientFromConn(context.Background(), conn, testMotorName, logger)

		pos, err := workingMotorDialedClient.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		features, err := workingMotorDialedClient.GetFeatures(context.Background(), nil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.GoTo(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.ResetZeroPosition(context.Background(), 0.5, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		isOn, err := workingMotorDialedClient.IsPowered(context.Background(), nil)
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, utils.TryClose(context.Background(), workingMotorDialedClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing motor", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingMotorDialedClient := motor.NewClientFromConn(context.Background(), conn, failMotorName, logger)

		err = failingMotorDialedClient.SetPower(context.Background(), 39.2, nil)
		test.That(t, err, test.ShouldNotBeNil)

		features, err := failingMotorDialedClient.GetFeatures(context.Background(), nil)
		test.That(t, features, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		isOn, err := failingMotorDialedClient.IsPowered(context.Background(), nil)
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		test.That(t, utils.TryClose(context.Background(), failingMotorDialedClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectMotor := &inject.Motor{}

	motorSvc, err := subtype.New(map[resource.Name]interface{}{motor.Named(testMotorName): injectMotor})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterMotorServiceServer(gServer, motor.NewServer(motorSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := motor.NewClientFromConn(ctx, conn1, testMotorName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := motor.NewClientFromConn(ctx, conn2, testMotorName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

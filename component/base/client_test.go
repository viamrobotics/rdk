package base_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func setupWorkingBase(
	workingBase *inject.Base,
	argsReceived map[string][]interface{},
	width int,
) {
	workingBase.MoveStraightFunc = func(
		ctx context.Context, distanceMm int,
		mmPerSec float64,
	) error {
		argsReceived["MoveStraight"] = []interface{}{distanceMm, mmPerSec}
		return nil
	}

	workingBase.SpinFunc = func(
		ctx context.Context, angleDeg, degsPerSec float64,
	) error {
		argsReceived["Spin"] = []interface{}{angleDeg, degsPerSec}
		return nil
	}

	workingBase.StopFunc = func(ctx context.Context) error {
		return nil
	}

	workingBase.GetWidthFunc = func(ctx context.Context) (int, error) {
		return width, nil
	}
}

func setupBrokenBase(brokenBase *inject.Base) string {
	errMsg := "critical failure"

	brokenBase.MoveStraightFunc = func(
		ctx context.Context,
		distanceMm int, mmPerSec float64,
	) error {
		return errors.New(errMsg)
	}
	brokenBase.SpinFunc = func(
		ctx context.Context,
		angleDeg, degsPerSec float64,
	) error {
		return errors.New(errMsg)
	}
	brokenBase.StopFunc = func(ctx context.Context) error {
		return errors.New(errMsg)
	}
	brokenBase.GetWidthFunc = func(ctx context.Context) (int, error) {
		return 0, errors.New(errMsg)
	}
	return errMsg
}

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	argsReceived := map[string][]interface{}{}

	workingBase := &inject.Base{}
	expectedWidth := 100
	setupWorkingBase(workingBase, argsReceived, expectedWidth)

	brokenBase := &inject.Base{}
	brokenBaseErrMsg := setupBrokenBase(brokenBase)

	resMap := map[resource.Name]interface{}{
		base.Named(testBaseName): workingBase,
		base.Named(failBaseName): brokenBase,
	}

	baseSvc, err := subtype.New(resMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(base.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, baseSvc)

	generic.RegisterService(rpcServer, baseSvc)
	workingBase.DoFunc = generic.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})
	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingBaseClient := base.NewClientFromConn(context.Background(), conn, testBaseName, logger)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), workingBaseClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()

	t.Run("working base client", func(t *testing.T) {
		distance := 42
		mmPerSec := 42.0
		err = workingBaseClient.MoveStraight(context.Background(), distance, mmPerSec)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs := []interface{}{distance, mmPerSec}
		test.That(t, argsReceived["MoveStraight"], test.ShouldResemble, expectedArgs)

		// Do
		resp, err := workingBaseClient.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		err = workingBaseClient.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("working base client by dialing", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testBaseName, logger)
		workingBaseClient2, ok := client.(base.Base)
		test.That(t, ok, test.ShouldBeTrue)

		degsPerSec := 42.0
		angleDeg := 30.0

		err = workingBaseClient2.Spin(context.Background(), angleDeg, degsPerSec)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs := []interface{}{angleDeg, degsPerSec}
		test.That(t, argsReceived["Spin"], test.ShouldResemble, expectedArgs)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("failing base client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingBaseClient := base.NewClientFromConn(context.Background(), conn, failBaseName, logger)
		test.That(t, err, test.ShouldBeNil)

		err = failingBaseClient.MoveStraight(context.Background(), 42, 42.0)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.Spin(context.Background(), 42.0, 42.0)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.Stop(context.Background())
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		test.That(t, utils.TryClose(context.Background(), failingBaseClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectBase := &inject.Base{}

	baseSvc, err := subtype.New(map[resource.Name]interface{}{base.Named(testBaseName): injectBase})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterBaseServiceServer(gServer, base.NewServer(baseSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := base.NewClientFromConn(ctx, conn1, testBaseName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := base.NewClientFromConn(ctx, conn2, testBaseName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

package base_test

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

	"go.viam.com/rdk/component/base"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
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
		mmPerSec float64, block bool,
	) error {
		argsReceived["MoveStraight"] = []interface{}{distanceMm, mmPerSec, block}
		return nil
	}

	workingBase.MoveArcFunc = func(
		ctx context.Context, distanceMm int,
		mmPerSec, angleDeg float64, block bool,
	) error {
		argsReceived["MoveArc"] = []interface{}{distanceMm, mmPerSec, angleDeg, block}
		return nil
	}

	workingBase.SpinFunc = func(
		ctx context.Context, angleDeg, degsPerSec float64, block bool,
	) error {
		argsReceived["Spin"] = []interface{}{angleDeg, degsPerSec, block}
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
		block bool) error {
		return errors.New(errMsg)
	}
	brokenBase.MoveArcFunc = func(
		ctx context.Context, distanceMm int,
		mmPerSec, angleDeg float64, block bool,
	) error {
		return errors.New(errMsg)
	}
	brokenBase.SpinFunc = func(
		ctx context.Context,
		angleDeg, degsPerSec float64, block bool,
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

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = base.NewClient(cancelCtx, testBaseName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingBaseClient, err := base.NewClient(context.Background(), testBaseName, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), workingBaseClient), test.ShouldBeNil)
	}()

	t.Run("working base client", func(t *testing.T) {
		distance := 42
		mmPerSec := 42.0
		shouldBlock := true
		err = workingBaseClient.MoveStraight(context.Background(), distance, mmPerSec, shouldBlock)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs := []interface{}{distance, mmPerSec, shouldBlock}
		test.That(t, argsReceived["MoveStraight"], test.ShouldResemble, expectedArgs)

		err = workingBaseClient.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("working base client by dialing", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testBaseName, logger)
		workingBaseClient2, ok := client.(base.Base)
		test.That(t, ok, test.ShouldBeTrue)

		distance := 42
		mmPerSec := 42.0
		degsPerSec := 42.0
		angleDeg := 30.0
		shouldBlock := true

		expectedArgs := []interface{}{distance, mmPerSec, angleDeg, shouldBlock}
		err = workingBaseClient2.MoveArc(context.Background(), distance, mmPerSec, angleDeg, shouldBlock)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, argsReceived["MoveArc"], test.ShouldResemble, expectedArgs)

		shouldBlock = false
		err = workingBaseClient2.Spin(context.Background(), angleDeg, degsPerSec, shouldBlock)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs = []interface{}{angleDeg, degsPerSec, shouldBlock}
		test.That(t, argsReceived["Spin"], test.ShouldResemble, expectedArgs)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("failing base client", func(t *testing.T) {
		failingBaseClient, err := base.NewClient(context.Background(), failBaseName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		err = failingBaseClient.MoveStraight(context.Background(), 42, 42.0, false)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.MoveArc(context.Background(), 42, 42.0, 42.0, false)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.Spin(context.Background(), 42.0, 42.0, true)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.Stop(context.Background())
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		test.That(t, utils.TryClose(context.Background(), failingBaseClient), test.ShouldBeNil)
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
	client1, err := base.NewClient(ctx, testBaseName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := base.NewClient(ctx, testBaseName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

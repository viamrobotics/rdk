package base_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/base"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
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
		extra map[string]interface{},
	) error {
		argsReceived["MoveStraight"] = []interface{}{distanceMm, mmPerSec, extra}
		return nil
	}

	workingBase.SpinFunc = func(
		ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{},
	) error {
		argsReceived["Spin"] = []interface{}{angleDeg, degsPerSec, extra}
		return nil
	}

	workingBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}

	workingBase.WidthFunc = func(ctx context.Context) (int, error) {
		return width, nil
	}
}

func setupBrokenBase(brokenBase *inject.Base) string {
	errMsg := "critical failure"

	brokenBase.MoveStraightFunc = func(
		ctx context.Context,
		distanceMm int, mmPerSec float64,
		extra map[string]interface{},
	) error {
		return errors.New(errMsg)
	}
	brokenBase.SpinFunc = func(
		ctx context.Context,
		angleDeg, degsPerSec float64,
		extra map[string]interface{},
	) error {
		return errors.New(errMsg)
	}
	brokenBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New(errMsg)
	}
	brokenBase.WidthFunc = func(ctx context.Context) (int, error) {
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

	resMap := map[resource.Name]base.Base{
		base.Named(testBaseName): workingBase,
		base.Named(failBaseName): brokenBase,
	}

	baseSvc, err := resource.NewAPIResourceCollection(base.API, resMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[base.Base](base.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, baseSvc), test.ShouldBeNil)

	workingBase.DoFunc = testutils.EchoFunc

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
	workingBaseClient, err := base.NewClientFromConn(context.Background(), conn, "", base.Named(testBaseName), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, workingBaseClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()

	t.Run("working base client", func(t *testing.T) {
		distance := 42
		mmPerSec := 42.0
		err = workingBaseClient.MoveStraight(
			context.Background(),
			distance,
			mmPerSec,
			map[string]interface{}{"foo": "bar"},
		)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs := []interface{}{distance, mmPerSec, map[string]interface{}{"foo": "bar"}}
		test.That(t, argsReceived["MoveStraight"], test.ShouldResemble, expectedArgs)

		// DoCommand
		resp, err := workingBaseClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		err = workingBaseClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("working base client by dialing", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", base.Named(testBaseName), logger)
		test.That(t, err, test.ShouldBeNil)

		degsPerSec := 42.0
		angleDeg := 30.0

		err = client.Spin(context.Background(), angleDeg, degsPerSec, nil)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs := []interface{}{angleDeg, degsPerSec, map[string]interface{}{}}
		test.That(t, argsReceived["Spin"], test.ShouldResemble, expectedArgs)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("failing base client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingBaseClient, err := base.NewClientFromConn(context.Background(), conn, "", base.Named(failBaseName), logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)

		err = failingBaseClient.MoveStraight(context.Background(), 42, 42.0, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.Spin(context.Background(), 42.0, 42.0, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		err = failingBaseClient.Stop(context.Background(), nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, brokenBaseErrMsg)

		test.That(t, failingBaseClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

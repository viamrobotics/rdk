package task_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	utilstestutils "go.viam.com/utils/testutils"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const testTaskName = "task1"

func setupTaskService(t *testing.T, injectTask *inject.Task) (net.Listener, func()) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	taskSvc, err := resource.NewAPIResourceCollection(
		task.API, map[resource.Name]task.Service{task.Named(testTaskName): injectTask},
	)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[task.Service](task.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, taskSvc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener)
	return listener, func() { rpcServer.Stop() }
}

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectTask := inject.NewTask(testTaskName)
	injectTask.Service = task.NewFromFunc(task.Named(testTaskName),
		func(ctx context.Context, input map[string]interface{}, report task.ProgressFunc) (task.Result, error) {
			report(task.Progress{Message: "step 1", Fraction: 0.25})
			if input["block"] == true {
				<-ctx.Done()
				return task.Result{}, ctx.Err()
			}
			if input["fail"] == true {
				return task.Result{}, errors.New("task failed")
			}
			return task.Result{Outcome: "custom", Output: map[string]interface{}{"echo": input["x"]}}, nil
		})
	injectTask.DoFunc = testutils.EchoFunc

	listener, stop := setupTaskService(t, injectTask)
	defer stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client, err := task.NewClientFromConn(context.Background(), conn, "", task.Named(testTaskName), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, conn.Close(), test.ShouldBeNil) }()

	t.Run("run streams progress then result", func(t *testing.T) {
		var progress []task.Progress
		res, err := client.Run(context.Background(), map[string]interface{}{"x": "y"}, func(p task.Progress) {
			progress = append(progress, p)
		}, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res.Outcome, test.ShouldEqual, "custom")
		test.That(t, res.Output, test.ShouldResemble, map[string]interface{}{"echo": "y"})
		test.That(t, progress, test.ShouldResemble, []task.Progress{{Message: "step 1", Fraction: 0.25}})

		st, err := client.GetStatus(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, st.Phase, test.ShouldEqual, task.PhaseIdle)
		test.That(t, st.LastResult.Outcome, test.ShouldEqual, "custom")
	})

	t.Run("run error", func(t *testing.T) {
		_, err := client.Run(context.Background(), map[string]interface{}{"fail": true}, nil, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "task failed")
	})

	t.Run("cancel and already running", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			_, err := client.Run(ctx, map[string]interface{}{"block": true}, nil, nil)
			errCh <- err
		}()
		utilstestutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			st, err := client.GetStatus(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, st.Phase, test.ShouldEqual, task.PhaseRunning)
			test.That(tb, st.StartedAt.IsZero(), test.ShouldBeFalse)
			test.That(tb, st.Progress, test.ShouldResemble, &task.Progress{Message: "step 1", Fraction: 0.25})
		})

		_, err := client.Run(context.Background(), nil, nil, nil)
		test.That(t, errors.Is(err, task.ErrAlreadyRunning), test.ShouldBeTrue)

		cancel()
		test.That(t, <-errCh, test.ShouldNotBeNil)
		utilstestutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			st, err := client.GetStatus(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, st.Phase, test.ShouldEqual, task.PhaseIdle)
		})
	})

	t.Run("do command", func(t *testing.T) {
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])
	})
}

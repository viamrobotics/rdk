package statemachine_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/statemachine"
	"go.viam.com/rdk/services/task"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const testName = "sm1"

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectSM := inject.NewStateMachine(testName)
	coll, err := resource.NewAPIResourceCollection(
		statemachine.API, map[resource.Name]statemachine.Service{statemachine.Named(testName): injectSM},
	)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[statemachine.Service](statemachine.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, coll, logger), test.ShouldBeNil)
	go rpcServer.Serve(listener)
	defer rpcServer.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client, err := statemachine.NewClientFromConn(context.Background(), conn, "", statemachine.Named(testName), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, conn.Close(), test.ShouldBeNil) }()
	ctx := context.Background()

	enteredAt := time.Now().UTC().Truncate(time.Millisecond)
	injectSM.GetStateFunc = func(ctx context.Context, extra map[string]interface{}) (statemachine.State, error) {
		test.That(t, extra["e"], test.ShouldEqual, 1.0)
		return statemachine.State{
			Path:         []string{"auto", "pour"},
			Status:       statemachine.StatusStopped,
			Context:      map[string]interface{}{"pour_ml": 150.0},
			EnteredAt:    enteredAt,
			TaskProgress: &task.Progress{Message: "m", Fraction: 0.5},
		}, nil
	}
	st, err := client.GetState(ctx, map[string]interface{}{"e": 1.0})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Path, test.ShouldResemble, []string{"auto", "pour"})
	test.That(t, st.Status, test.ShouldEqual, statemachine.StatusStopped)
	test.That(t, st.Context, test.ShouldResemble, map[string]interface{}{"pour_ml": 150.0})
	test.That(t, st.EnteredAt.Equal(enteredAt), test.ShouldBeTrue)
	test.That(t, st.TaskProgress, test.ShouldResemble, &task.Progress{Message: "m", Fraction: 0.5})

	injectSM.GetStateFunc = func(ctx context.Context, extra map[string]interface{}) (statemachine.State, error) {
		return statemachine.State{Path: []string{"idle"}}, nil
	}
	st, err = client.GetState(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Status, test.ShouldEqual, statemachine.StatusRunning)
	test.That(t, st.EnteredAt.IsZero(), test.ShouldBeTrue)
	test.That(t, st.TaskProgress, test.ShouldBeNil)

	injectSM.SendEventFunc = func(
		ctx context.Context, event string, payload, extra map[string]interface{},
	) (statemachine.EventResult, error) {
		test.That(t, event, test.ShouldEqual, "pour_button")
		test.That(t, payload, test.ShouldResemble, map[string]interface{}{"glass": "a"})
		return statemachine.EventResult{Handled: true, Path: []string{"manual", "pour"}}, nil
	}
	res, err := client.SendEvent(ctx, "pour_button", map[string]interface{}{"glass": "a"}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, statemachine.EventResult{Handled: true, Path: []string{"manual", "pour"}})

	ts := time.Now().UTC().Truncate(time.Millisecond)
	injectSM.GetHistoryFunc = func(ctx context.Context, limit int, extra map[string]interface{}) ([]statemachine.Transition, error) {
		test.That(t, limit, test.ShouldEqual, 3)
		return []statemachine.Transition{
			{From: []string{}, To: []string{"idle"}, Trigger: "initial", Timestamp: ts},
			{
				From: []string{"idle"}, To: []string{"work"}, Trigger: "go", Timestamp: ts,
				TaskResult: &task.Result{Outcome: "done", Output: map[string]interface{}{"k": "v"}},
			},
		}, nil
	}
	history, err := client.GetHistory(ctx, 3, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history, test.ShouldHaveLength, 2)
	test.That(t, history[0].Trigger, test.ShouldEqual, "initial")
	test.That(t, history[0].TaskResult, test.ShouldBeNil)
	test.That(t, history[0].Timestamp.Equal(ts), test.ShouldBeTrue)
	test.That(t, history[1].From, test.ShouldResemble, []string{"idle"})
	test.That(t, history[1].TaskResult, test.ShouldResemble, &task.Result{Outcome: "done", Output: map[string]interface{}{"k": "v"}})

	injectSM.GetDefinitionFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"initial": "idle"}, nil
	}
	def, err := client.GetDefinition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, def, test.ShouldResemble, map[string]interface{}{"initial": "idle"})

	var stopped, reset bool
	injectSM.StopFunc = func(ctx context.Context, extra map[string]interface{}) error { stopped = true; return nil }
	injectSM.ResetFunc = func(ctx context.Context, extra map[string]interface{}) error { reset = true; return nil }
	test.That(t, client.Stop(ctx, nil), test.ShouldBeNil)
	test.That(t, client.Reset(ctx, nil), test.ShouldBeNil)
	test.That(t, stopped, test.ShouldBeTrue)
	test.That(t, reset, test.ShouldBeTrue)

	injectSM.ResetFunc = func(ctx context.Context, extra map[string]interface{}) error { return errors.New("no") }
	err = client.Reset(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no")

	injectSM.DoFunc = testutils.EchoFunc
	resp, err := client.DoCommand(ctx, testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])

	status, err := client.Status(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, map[string]interface{}{})
}

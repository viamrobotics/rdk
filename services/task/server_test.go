package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "go.viam.com/api/service/task/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
	"go.viam.com/rdk/testutils/inject"
)

type testRunServer struct {
	grpc.ServerStream
	ctx  context.Context
	sent []*pb.RunResponse
}

func (x *testRunServer) Context() context.Context { return x.ctx }

func (x *testRunServer) Send(m *pb.RunResponse) error {
	x.sent = append(x.sent, m)
	return nil
}

func newServer(t *testing.T) (pb.TaskServiceServer, *inject.Task) {
	t.Helper()
	injectTask := inject.NewTask("task1")
	coll, err := resource.NewAPIResourceCollection(task.API, map[resource.Name]task.Service{task.Named("task1"): injectTask})
	test.That(t, err, test.ShouldBeNil)
	return task.NewRPCServiceServer(coll, logging.NewTestLogger(t)).(pb.TaskServiceServer), injectTask
}

func TestServerRun(t *testing.T) {
	server, injectTask := newServer(t)

	t.Run("not found", func(t *testing.T) {
		err := server.Run(&pb.RunRequest{Name: "nope"}, &testRunServer{ctx: context.Background()})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	})

	t.Run("progress and result", func(t *testing.T) {
		injectTask.RunFunc = func(
			ctx context.Context, input map[string]interface{}, onProgress task.ProgressFunc, extra map[string]interface{},
		) (task.Result, error) {
			test.That(t, input["a"], test.ShouldEqual, 1.0)
			test.That(t, extra["e"], test.ShouldEqual, true)
			onProgress(task.Progress{Message: "m", Fraction: 0.5})
			return task.Result{Outcome: "no_glass", Output: map[string]interface{}{"k": "v"}}, nil
		}
		input, err := protoutils.StructToStructPb(map[string]interface{}{"a": 1.0})
		test.That(t, err, test.ShouldBeNil)
		extra, err := protoutils.StructToStructPb(map[string]interface{}{"e": true})
		test.That(t, err, test.ShouldBeNil)
		stream := &testRunServer{ctx: context.Background()}
		err = server.Run(&pb.RunRequest{Name: "task1", Input: input, Extra: extra}, stream)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stream.sent, test.ShouldHaveLength, 2)
		test.That(t, stream.sent[0].GetProgress().Message, test.ShouldEqual, "m")
		test.That(t, stream.sent[1].GetResult().Outcome, test.ShouldEqual, "no_glass")
		test.That(t, stream.sent[1].GetResult().Output.AsMap(), test.ShouldResemble, map[string]interface{}{"k": "v"})
	})

	t.Run("already running maps to failed precondition", func(t *testing.T) {
		injectTask.RunFunc = func(
			ctx context.Context, input map[string]interface{}, onProgress task.ProgressFunc, extra map[string]interface{},
		) (task.Result, error) {
			return task.Result{}, task.ErrAlreadyRunning
		}
		err := server.Run(&pb.RunRequest{Name: "task1"}, &testRunServer{ctx: context.Background()})
		test.That(t, status.Code(err), test.ShouldEqual, codes.FailedPrecondition)
	})

	t.Run("other errors pass through", func(t *testing.T) {
		injectTask.RunFunc = func(
			ctx context.Context, input map[string]interface{}, onProgress task.ProgressFunc, extra map[string]interface{},
		) (task.Result, error) {
			return task.Result{}, errors.New("boom")
		}
		err := server.Run(&pb.RunRequest{Name: "task1"}, &testRunServer{ctx: context.Background()})
		test.That(t, err, test.ShouldBeError, errors.New("boom"))
	})
}

func TestServerGetStatus(t *testing.T) {
	server, injectTask := newServer(t)
	startedAt := time.Now()
	injectTask.GetStatusFunc = func(ctx context.Context, extra map[string]interface{}) (task.Status, error) {
		return task.Status{
			Phase:      task.PhaseRunning,
			StartedAt:  startedAt,
			Progress:   &task.Progress{Message: "p", Fraction: 0.1},
			LastResult: &task.Result{Outcome: "done", Output: map[string]interface{}{"x": 1.0}},
		}, nil
	}
	resp, err := server.GetStatus(context.Background(), &pb.GetStatusRequest{Name: "task1"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Phase, test.ShouldEqual, pb.Phase_PHASE_RUNNING)
	test.That(t, resp.StartedAt.AsTime().Equal(startedAt), test.ShouldBeTrue)
	test.That(t, resp.Progress.Message, test.ShouldEqual, "p")
	test.That(t, resp.LastResult.Outcome, test.ShouldEqual, "done")
	test.That(t, resp.LastResult.Output.AsMap(), test.ShouldResemble, map[string]interface{}{"x": 1.0})

	injectTask.GetStatusFunc = func(ctx context.Context, extra map[string]interface{}) (task.Status, error) {
		return task.Status{}, nil
	}
	resp, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{Name: "task1"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Phase, test.ShouldEqual, pb.Phase_PHASE_IDLE)
	test.That(t, resp.StartedAt, test.ShouldBeNil)
	test.That(t, resp.LastResult, test.ShouldBeNil)

	_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{Name: "nope"})
	test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
}

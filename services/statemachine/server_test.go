package statemachine_test

import (
	"context"
	"testing"
	"time"

	pb "go.viam.com/api/service/statemachine/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/statemachine"
	"go.viam.com/rdk/services/task"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(t *testing.T) (pb.StateMachineServiceServer, *inject.StateMachine) {
	t.Helper()
	injectSM := inject.NewStateMachine("sm1")
	coll, err := resource.NewAPIResourceCollection(
		statemachine.API, map[resource.Name]statemachine.Service{statemachine.Named("sm1"): injectSM},
	)
	test.That(t, err, test.ShouldBeNil)
	return statemachine.NewRPCServiceServer(coll, logging.NewTestLogger(t)).(pb.StateMachineServiceServer), injectSM
}

func TestServer(t *testing.T) {
	server, injectSM := newServer(t)
	ctx := context.Background()

	_, err := server.GetState(ctx, &pb.GetStateRequest{Name: "nope"})
	test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)

	enteredAt := time.Now()
	injectSM.GetStateFunc = func(ctx context.Context, extra map[string]interface{}) (statemachine.State, error) {
		return statemachine.State{
			Path: []string{"a", "b"}, Status: statemachine.StatusFaulted, Context: map[string]interface{}{"x": 1.0},
			EnteredAt: enteredAt, TaskProgress: &task.Progress{Message: "p", Fraction: 0.25}, JobID: "job-9",
		}, nil
	}
	stResp, err := server.GetState(ctx, &pb.GetStateRequest{Name: "sm1"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stResp.JobId, test.ShouldEqual, "job-9")
	test.That(t, stResp.Path, test.ShouldResemble, []string{"a", "b"})
	test.That(t, stResp.Status, test.ShouldEqual, pb.Status_STATUS_FAULTED)
	test.That(t, stResp.Context.AsMap(), test.ShouldResemble, map[string]interface{}{"x": 1.0})
	test.That(t, stResp.EnteredAt.AsTime().Equal(enteredAt), test.ShouldBeTrue)
	test.That(t, stResp.TaskProgress.Fraction, test.ShouldEqual, 0.25)

	injectSM.SendEventFunc = func(
		ctx context.Context, event string, payload, extra map[string]interface{},
	) (statemachine.EventResult, error) {
		return statemachine.EventResult{Handled: event == "go", Path: []string{"a"}}, nil
	}
	evResp, err := server.SendEvent(ctx, &pb.SendEventRequest{Name: "sm1", Event: "go"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, evResp.Handled, test.ShouldBeTrue)
	test.That(t, evResp.Path, test.ShouldResemble, []string{"a"})
	evResp, err = server.SendEvent(ctx, &pb.SendEventRequest{Name: "sm1", Event: "other"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, evResp.Handled, test.ShouldBeFalse)

	injectSM.GetHistoryFunc = func(
		ctx context.Context, jobID string, limit int, extra map[string]interface{},
	) ([]statemachine.JobHistory, error) {
		test.That(t, jobID, test.ShouldEqual, "job-9")
		test.That(t, limit, test.ShouldEqual, 2)
		return []statemachine.JobHistory{{JobID: "job-9", StartedAt: enteredAt, Transitions: []statemachine.Transition{{
			From: []string{}, To: []string{"a"}, Trigger: "initial", Source: statemachine.SourceInitial, Timestamp: enteredAt,
			TaskResult: &task.Result{Outcome: "done"},
		}}}}, nil
	}
	hResp, err := server.GetHistory(ctx, &pb.GetHistoryRequest{Name: "sm1", JobId: "job-9", Limit: 2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hResp.Jobs, test.ShouldHaveLength, 1)
	test.That(t, hResp.Jobs[0].JobId, test.ShouldEqual, "job-9")
	test.That(t, hResp.Jobs[0].StartedAt.AsTime().Equal(enteredAt), test.ShouldBeTrue)
	test.That(t, hResp.Jobs[0].Transitions, test.ShouldHaveLength, 1)
	test.That(t, hResp.Jobs[0].Transitions[0].To, test.ShouldResemble, []string{"a"})
	test.That(t, hResp.Jobs[0].Transitions[0].TaskResult.Outcome, test.ShouldEqual, "done")
	test.That(t, hResp.Jobs[0].Transitions[0].Source, test.ShouldEqual, pb.TransitionSource_TRANSITION_SOURCE_INITIAL)

	injectSM.GetDefinitionFunc = func(ctx context.Context, extra map[string]interface{}) (statemachine.Definition, error) {
		return statemachine.Definition{
			Initial: "a",
			States:  []statemachine.StateDefinition{{Path: []string{"a"}, On: map[string]string{"x": "a"}, Task: "a-task", Timeout: time.Minute}},
			Events:  []string{"x"},
			On:      map[string]string{},
		}, nil
	}
	dResp, err := server.GetDefinition(ctx, &pb.GetDefinitionRequest{Name: "sm1"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dResp.Initial, test.ShouldEqual, "a")
	test.That(t, dResp.Events, test.ShouldResemble, []string{"x"})
	test.That(t, dResp.States, test.ShouldHaveLength, 1)
	test.That(t, dResp.States[0].Path, test.ShouldResemble, []string{"a"})
	test.That(t, dResp.States[0].On, test.ShouldResemble, map[string]string{"x": "a"})
	test.That(t, dResp.States[0].Task, test.ShouldEqual, "a-task")
	test.That(t, dResp.States[0].Timeout.AsDuration(), test.ShouldEqual, time.Minute)

	injectSM.CurrentJobIDFunc = func(ctx context.Context, extra map[string]interface{}) (string, error) {
		return "job-9", nil
	}
	jResp, err := server.GetCurrentJobId(ctx, &pb.GetCurrentJobIdRequest{Name: "sm1"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jResp.JobId, test.ShouldEqual, "job-9")

	var stopped, reset bool
	injectSM.StopFunc = func(ctx context.Context, extra map[string]interface{}) error { stopped = true; return nil }
	injectSM.ResetFunc = func(ctx context.Context, extra map[string]interface{}) error { reset = true; return nil }
	_, err = server.Stop(ctx, &pb.StopRequest{Name: "sm1"})
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Reset(ctx, &pb.ResetRequest{Name: "sm1"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stopped, test.ShouldBeTrue)
	test.That(t, reset, test.ShouldBeTrue)
}

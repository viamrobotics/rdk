package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/services/task"
)

func TestRunnerOneAtATime(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	svc := task.NewFromFunc(task.Named("t"), func(
		ctx context.Context, input map[string]interface{}, report task.ProgressFunc,
	) (task.Result, error) {
		report(task.Progress{Message: "half", Fraction: 0.5})
		close(started)
		select {
		case <-release:
			return task.Done(map[string]interface{}{"echo": input["x"]}), nil
		case <-ctx.Done():
			return task.Result{}, ctx.Err()
		}
	})

	st, err := svc.GetStatus(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Phase, test.ShouldEqual, task.PhaseIdle)
	test.That(t, st.LastResult, test.ShouldBeNil)

	resultCh := make(chan task.Result, 1)
	go func() {
		res, err := svc.Run(context.Background(), map[string]interface{}{"x": 1.0}, nil)
		test.That(t, err, test.ShouldBeNil)
		resultCh <- res
	}()
	<-started

	st, err = svc.GetStatus(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Phase, test.ShouldEqual, task.PhaseRunning)
	test.That(t, st.StartedAt.IsZero(), test.ShouldBeFalse)
	test.That(t, st.Progress, test.ShouldResemble, &task.Progress{Message: "half", Fraction: 0.5})

	_, err = svc.Run(context.Background(), nil, nil)
	test.That(t, errors.Is(err, task.ErrAlreadyRunning), test.ShouldBeTrue)

	close(release)
	res := <-resultCh
	test.That(t, res.Outcome, test.ShouldEqual, task.OutcomeDone)
	test.That(t, res.Output["echo"], test.ShouldEqual, 1.0)

	st, err = svc.GetStatus(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Phase, test.ShouldEqual, task.PhaseIdle)
	test.That(t, st.Progress, test.ShouldBeNil)
	test.That(t, st.LastResult.Outcome, test.ShouldEqual, task.OutcomeDone)
}

func TestRunnerErrorAndCancel(t *testing.T) {
	errBoom := errors.New("boom")
	svc := task.NewFromFunc(task.Named("t"), func(
		ctx context.Context, input map[string]interface{}, report task.ProgressFunc,
	) (task.Result, error) {
		if input["fail"] == true {
			return task.Result{}, errBoom
		}
		<-ctx.Done()
		return task.Result{}, ctx.Err()
	})

	_, err := svc.Run(context.Background(), map[string]interface{}{"fail": true}, nil)
	test.That(t, errors.Is(err, errBoom), test.ShouldBeTrue)
	st, err := svc.GetStatus(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.LastResult.Outcome, test.ShouldEqual, task.OutcomeError)
	test.That(t, st.LastResult.Output["error"], test.ShouldEqual, "boom")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, nil, nil)
		errCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	test.That(t, errors.Is(<-errCh, context.Canceled), test.ShouldBeTrue)

	go func() {
		_, err := svc.Run(context.Background(), nil, nil)
		errCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
	test.That(t, errors.Is(<-errCh, context.Canceled), test.ShouldBeTrue)
}

package builtin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/statemachine"
	"go.viam.com/rdk/services/task"
	"go.viam.com/rdk/testutils/inject"
)

// controlledTask is an inject task whose executions block until released.
type controlledTask struct {
	*inject.Task
	started   chan struct{}
	release   chan task.Result
	runs      atomic.Int32
	lastInput map[string]interface{}
}

func newControlledTask(name string) *controlledTask {
	ct := &controlledTask{Task: inject.NewTask(name), started: make(chan struct{}, 16), release: make(chan task.Result)}
	ct.RunFunc = func(
		ctx context.Context, input map[string]interface{}, onProgress task.ProgressFunc, _ map[string]interface{},
	) (task.Result, error) {
		ct.runs.Add(1)
		ct.lastInput = input
		ct.started <- struct{}{}
		onProgress(task.Progress{Message: "working", Fraction: 0.5})
		select {
		case <-ctx.Done():
			return task.Result{}, ctx.Err()
		case res := <-ct.release:
			if res.Outcome == "fail" {
				return task.Result{}, errors.New("task failed")
			}
			return res, nil
		}
	}
	return ct
}

func (ct *controlledTask) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-ct.started:
	case <-time.After(5 * time.Second):
		t.Fatal("task did not start")
	}
}

func (ct *controlledTask) finish(t *testing.T, res task.Result) {
	t.Helper()
	select {
	case ct.release <- res:
	case <-time.After(5 * time.Second):
		t.Fatal("task was not waiting to be released")
	}
}

func newMachine(t *testing.T, cfg *Config, deps resource.Dependencies) statemachine.Service {
	t.Helper()
	_, _, err := cfg.Validate("")
	test.That(t, err, test.ShouldBeNil)
	svc, err := NewBuiltIn(context.Background(), deps, resource.Config{
		Name:                "sm",
		API:                 statemachine.API,
		ConvertedAttributes: cfg,
		Attributes:          map[string]interface{}{"initial": cfg.Initial},
	}, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, svc.Close(context.Background()), test.ShouldBeNil) })
	return svc
}

func waitForPath(t *testing.T, svc statemachine.Service, path ...string) statemachine.State {
	t.Helper()
	var st statemachine.State
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		var err error
		st, err = svc.GetState(context.Background(), nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, st.Path, test.ShouldResemble, path)
	})
	return st
}

func waitForStatus(t *testing.T, svc statemachine.Service, status statemachine.Status) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		st, err := svc.GetState(context.Background(), nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, st.Status, test.ShouldEqual, status)
	})
}

func resourceTask(name string) *TaskConfig {
	return &TaskConfig{Type: TaskTypeResource, Name: name}
}

func TestFlatMachine(t *testing.T) {
	work := newControlledTask("work")
	deps := resource.Dependencies{work.Name(): work}
	cfg := &Config{
		Initial: "idle",
		Context: map[string]interface{}{"pour_ml": 150.0},
		On:      map[string]string{"error": "faulted"},
		States: map[string]*StateConfig{
			"idle":    {On: map[string]string{"go": "working"}},
			"working": {Task: resourceTask("work"), On: map[string]string{"done": "idle", "no_glass": "idle"}},
			"faulted": {On: map[string]string{"ack": "idle"}},
		},
	}
	deps2, _, err := cfg.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps2, test.ShouldResemble, []string{task.Named("work").String()})

	svc := newMachine(t, cfg, deps)
	ctx := context.Background()

	st := waitForPath(t, svc, "idle")
	test.That(t, st.Status, test.ShouldEqual, statemachine.StatusRunning)
	test.That(t, st.Context, test.ShouldResemble, map[string]interface{}{"pour_ml": 150.0})
	test.That(t, st.EnteredAt.IsZero(), test.ShouldBeFalse)
	test.That(t, st.TaskProgress, test.ShouldBeNil)

	res, err := svc.SendEvent(ctx, "nothing", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Handled, test.ShouldBeFalse)
	test.That(t, res.Path, test.ShouldResemble, []string{"idle"})

	res, err = svc.SendEvent(ctx, "go", map[string]interface{}{"glass": "a"}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Handled, test.ShouldBeTrue)
	test.That(t, res.Path, test.ShouldResemble, []string{"working"})
	work.waitStarted(t)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		st, err := svc.GetState(ctx, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, st.TaskProgress, test.ShouldResemble, &task.Progress{Message: "working", Fraction: 0.5})
	})

	work.finish(t, task.Done(map[string]interface{}{"glass_pose": "p"}))
	st = waitForPath(t, svc, "idle")
	test.That(t, st.Context, test.ShouldResemble, map[string]interface{}{"pour_ml": 150.0, "glass_pose": "p"})

	history, err := svc.GetHistory(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history, test.ShouldHaveLength, 3)
	test.That(t, history[0].From, test.ShouldResemble, []string{})
	test.That(t, history[0].To, test.ShouldResemble, []string{"idle"})
	test.That(t, history[0].Trigger, test.ShouldEqual, statemachine.TriggerInitial)
	test.That(t, history[1].Trigger, test.ShouldEqual, "go")
	test.That(t, history[2].From, test.ShouldResemble, []string{"working"})
	test.That(t, history[2].Trigger, test.ShouldEqual, "done")
	test.That(t, history[2].TaskResult.Output["glass_pose"], test.ShouldEqual, "p")
	history, err = svc.GetHistory(ctx, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history, test.ShouldHaveLength, 1)
	test.That(t, history[0].Trigger, test.ShouldEqual, "done")

	// A Go error becomes the outcome "error", which bubbles to the machine-level handler.
	_, err = svc.SendEvent(ctx, "go", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	work.waitStarted(t)
	work.finish(t, task.Result{Outcome: "fail"})
	st = waitForPath(t, svc, "faulted")
	test.That(t, st.Status, test.ShouldEqual, statemachine.StatusRunning)

	def, err := svc.GetDefinition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, def["initial"], test.ShouldEqual, "idle")

	status, err := svc.Status(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status["status"], test.ShouldEqual, "running")
	test.That(t, status["path"], test.ShouldResemble, []interface{}{"faulted"})
}

func TestUnhandledErrorFaultsAndResetRecovers(t *testing.T) {
	work := newControlledTask("work")
	deps := resource.Dependencies{work.Name(): work}
	cfg := &Config{
		Initial: "working",
		Context: map[string]interface{}{"n": 1.0},
		States: map[string]*StateConfig{
			"working": {Task: resourceTask("work"), On: map[string]string{"done": "idle"}},
			"idle":    {On: map[string]string{"go": "working"}},
		},
	}
	svc := newMachine(t, cfg, deps)
	ctx := context.Background()

	work.waitStarted(t)
	work.finish(t, task.Result{Outcome: "fail"})
	waitForStatus(t, svc, statemachine.StatusFaulted)
	st, err := svc.GetState(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Path, test.ShouldResemble, []string{"working"})

	// Faulted: nothing runs until Reset or an event with a transition out.
	res, err := svc.SendEvent(ctx, "go", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Handled, test.ShouldBeFalse)
	test.That(t, work.runs.Load(), test.ShouldEqual, 1)

	test.That(t, svc.Reset(ctx, nil), test.ShouldBeNil)
	waitForStatus(t, svc, statemachine.StatusRunning)
	work.waitStarted(t)
	test.That(t, work.runs.Load(), test.ShouldEqual, 2)
	work.finish(t, task.Done(nil))
	waitForPath(t, svc, "idle")

	history, err := svc.GetHistory(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history[1].Trigger, test.ShouldEqual, statemachine.TriggerFault)
	test.That(t, history[2].Trigger, test.ShouldEqual, statemachine.TriggerReset)
}

func TestRetryThenErrorTransition(t *testing.T) {
	work := newControlledTask("work")
	deps := resource.Dependencies{work.Name(): work}
	cfg := &Config{
		Initial: "working",
		States: map[string]*StateConfig{
			"working": {
				Task:  resourceTask("work"),
				Retry: &RetryConfig{Max: 2, Backoff: "1ms"},
				On:    map[string]string{"done": "idle", "error": "recover"},
			},
			"idle":    {},
			"recover": {},
		},
	}
	svc := newMachine(t, cfg, deps)

	for i := 0; i < 3; i++ {
		work.waitStarted(t)
		work.finish(t, task.Result{Outcome: "fail"})
	}
	waitForPath(t, svc, "recover")
	test.That(t, work.runs.Load(), test.ShouldEqual, 3)
	history, err := svc.GetHistory(context.Background(), 1, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history[0].Trigger, test.ShouldEqual, task.OutcomeError)
	test.That(t, history[0].TaskResult.Output["error"], test.ShouldEqual, "task failed")
}

func TestTimeout(t *testing.T) {
	work := newControlledTask("work")
	deps := resource.Dependencies{work.Name(): work}
	cfg := &Config{
		Initial: "working",
		States: map[string]*StateConfig{
			"working": {Task: resourceTask("work"), Timeout: "20ms", On: map[string]string{"timeout": "late", "done": "idle"}},
			"late":    {},
			"idle":    {},
		},
	}
	svc := newMachine(t, cfg, deps)
	work.waitStarted(t)
	waitForPath(t, svc, "late")
	history, err := svc.GetHistory(context.Background(), 1, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history[0].Trigger, test.ShouldEqual, task.OutcomeTimeout)
}

func TestPreemption(t *testing.T) {
	work := newControlledTask("work")
	deps := resource.Dependencies{work.Name(): work}
	cfg := &Config{
		Initial:       "working",
		CancelTimeout: "200ms",
		On:            map[string]string{"estop": "faulted"},
		States: map[string]*StateConfig{
			"working": {Task: resourceTask("work"), On: map[string]string{"done": "idle"}},
			"idle":    {},
			"faulted": {},
		},
	}
	svc := newMachine(t, cfg, deps)
	ctx := context.Background()
	work.waitStarted(t)

	// The task honors cancellation: the event preempts it and the next state enters.
	res, err := svc.SendEvent(ctx, "estop", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Handled, test.ShouldBeTrue)
	test.That(t, res.Path, test.ShouldResemble, []string{"faulted"})
	st, err := svc.GetState(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Status, test.ShouldEqual, statemachine.StatusRunning)
	test.That(t, st.TaskProgress, test.ShouldBeNil)
}

func TestPreemptionTimeoutFaults(t *testing.T) {
	stubborn := inject.NewTask("stubborn")
	release := make(chan struct{})
	stubborn.RunFunc = func(
		ctx context.Context, _ map[string]interface{}, _ task.ProgressFunc, _ map[string]interface{},
	) (task.Result, error) {
		<-release
		return task.Done(nil), nil
	}
	defer close(release)
	deps := resource.Dependencies{stubborn.Name(): stubborn}
	cfg := &Config{
		Initial:       "working",
		CancelTimeout: "20ms",
		States: map[string]*StateConfig{
			"working": {Task: resourceTask("stubborn"), On: map[string]string{"stop_it": "idle"}},
			"idle":    {},
		},
	}
	svc := newMachine(t, cfg, deps)
	ctx := context.Background()
	res, err := svc.SendEvent(ctx, "stop_it", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Handled, test.ShouldBeTrue)
	test.That(t, res.Path, test.ShouldResemble, []string{"working"})
	waitForStatus(t, svc, statemachine.StatusFaulted)
}

func TestStopHoldsUntilEvent(t *testing.T) {
	work := newControlledTask("work")
	deps := resource.Dependencies{work.Name(): work}
	cfg := &Config{
		Initial: "working",
		States: map[string]*StateConfig{
			"working": {
				Task: &TaskConfig{Type: TaskTypeResource, Name: "work", Input: map[string]interface{}{"side": "black", "ml": "$ctx.pour_ml"}},
				On:   map[string]string{"done": "idle", "again": "working"},
			},
			"idle": {},
		},
		Context: map[string]interface{}{"pour_ml": 150.0},
	}
	svc := newMachine(t, cfg, deps)
	ctx := context.Background()
	work.waitStarted(t)
	test.That(t, work.lastInput, test.ShouldResemble, map[string]interface{}{"pour_ml": 150.0, "side": "black", "ml": 150.0})

	test.That(t, svc.Stop(ctx, nil), test.ShouldBeNil)
	st, err := svc.GetState(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Status, test.ShouldEqual, statemachine.StatusStopped)
	test.That(t, st.Path, test.ShouldResemble, []string{"working"})
	test.That(t, st.TaskProgress, test.ShouldBeNil)
	test.That(t, work.runs.Load(), test.ShouldEqual, 1)

	res, err := svc.SendEvent(ctx, "again", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Handled, test.ShouldBeTrue)
	work.waitStarted(t)
	waitForStatus(t, svc, statemachine.StatusRunning)
	work.finish(t, task.Done(nil))
	waitForPath(t, svc, "idle")
}

// wineBot is the spec's example: two modes sharing one sub-machine, errors
// bubbling to recover, $history back to the mode, $done out of the sequence.
func wineBot() *Config {
	return &Config{
		Initial: "manual",
		Context: map[string]interface{}{"pour_ml": 150.0},
		On: map[string]string{
			"set_mode.manual": "manual",
			"set_mode.auto":   "auto",
			"estop":           "faulted",
			"error":           "recover",
		},
		States: map[string]*StateConfig{
			"manual": {
				Initial: "idle",
				States: map[string]*StateConfig{
					"idle": {On: map[string]string{"pour_button": "pour"}},
					"pour": {Machine: "pour_sequence", On: map[string]string{"done": "idle"}},
				},
			},
			"auto": {
				Initial: "watching",
				States: map[string]*StateConfig{
					"watching": {Task: resourceTask("watch"), On: map[string]string{"satisfied": "pour"}},
					"pour":     {Machine: "pour_sequence", On: map[string]string{"done": "watching"}},
				},
			},
			"recover": {Task: resourceTask("safe_home"), On: map[string]string{"done": "$history", "error": "faulted"}},
			"faulted": {On: map[string]string{"ack": "manual"}},
		},
		Machines: map[string]*MachineConfig{
			"pour_sequence": {
				Initial: "picking_glass",
				States: map[string]*StateConfig{
					"picking_glass": {Task: resourceTask("pick_glass"), On: map[string]string{"done": "pouring", "no_glass": "$done"}},
					"pouring":       {Task: resourceTask("pour"), On: map[string]string{"done": "$done"}},
				},
			},
		},
	}
}

func TestNestedMachine(t *testing.T) {
	watch := newControlledTask("watch")
	pick := newControlledTask("pick_glass")
	pour := newControlledTask("pour")
	home := newControlledTask("safe_home")
	deps := resource.Dependencies{watch.Name(): watch, pick.Name(): pick, pour.Name(): pour, home.Name(): home}
	cfg := wineBot()
	validated, _, err := cfg.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, validated, test.ShouldHaveLength, 4)

	svc := newMachine(t, cfg, deps)
	ctx := context.Background()
	waitForPath(t, svc, "manual", "idle")

	// Manual pour: button -> sub-machine -> $done -> idle.
	res, err := svc.SendEvent(ctx, "pour_button", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Path, test.ShouldResemble, []string{"manual", "pour", "picking_glass"})
	pick.waitStarted(t)
	pick.finish(t, task.Done(map[string]interface{}{"glass_pose": "g"}))
	waitForPath(t, svc, "manual", "pour", "pouring")
	pour.waitStarted(t)
	pour.finish(t, task.Done(nil))
	st := waitForPath(t, svc, "manual", "idle")
	test.That(t, st.Context["glass_pose"], test.ShouldEqual, "g")

	// no_glass exits the sequence early with the sequence's outcome; manual.pour has no
	// no_glass handler so the machine waits there.
	_, err = svc.SendEvent(ctx, "pour_button", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	pick.waitStarted(t)
	pick.finish(t, task.Result{Outcome: "no_glass"})
	waitForPath(t, svc, "manual", "pour", "picking_glass")
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		st, err := svc.GetState(ctx, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, st.TaskProgress, test.ShouldBeNil)
	})
	test.That(t, pick.runs.Load(), test.ShouldEqual, 2)

	// Switching modes preempts whatever child is running.
	res, err = svc.SendEvent(ctx, "set_mode.auto", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Path, test.ShouldResemble, []string{"auto", "watching"})
	watch.waitStarted(t)
	watch.finish(t, task.Result{Outcome: task.OutcomeSatisfied})
	waitForPath(t, svc, "auto", "pour", "picking_glass")
	pick.waitStarted(t)

	// An error deep in the sequence bubbles to the machine-level handler; recover
	// returns through $history to where auto left off.
	pick.finish(t, task.Result{Outcome: "fail"})
	waitForPath(t, svc, "recover")
	home.waitStarted(t)
	home.finish(t, task.Done(nil))
	waitForPath(t, svc, "auto", "pour", "picking_glass")
	pick.waitStarted(t)

	// estop from anywhere; ack back to manual's initial state.
	res, err = svc.SendEvent(ctx, "estop", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Path, test.ShouldResemble, []string{"faulted"})
	res, err = svc.SendEvent(ctx, "ack", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Path, test.ShouldResemble, []string{"manual", "idle"})

	history, err := svc.GetHistory(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)
	var triggers []string
	for _, h := range history {
		triggers = append(triggers, h.Trigger)
	}
	test.That(t, triggers, test.ShouldResemble, []string{
		"initial", "pour_button", "done", "done", "pour_button",
		"set_mode.auto", "satisfied", "error", "done", "estop", "ack",
	})
}

func TestBuiltinTasksAndWatchers(t *testing.T) {
	var pressed atomic.Bool
	button := inject.NewSensor("button")
	button.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"pressed": pressed.Load()}, nil
	}
	var glasses atomic.Int32
	vision := inject.NewSensor("glass_vision")
	vision.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		dets := []interface{}{}
		for i := int32(0); i < glasses.Load(); i++ {
			dets = append(dets, map[string]interface{}{"class_name": "empty_glass", "confidence": 0.9})
		}
		return map[string]interface{}{"detections": dets}, nil
	}
	var commands []map[string]interface{}
	controller := inject.NewGenericService("pour_controller")
	controller.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		commands = append(commands, cmd)
		return map[string]interface{}{"outcome": "poured", "ml": cmd["ml"]}, nil
	}
	deps := resource.Dependencies{button.Name(): button, vision.Name(): vision, controller.Name(): controller}

	cfg := &Config{
		Initial: "watching",
		Context: map[string]interface{}{"pour_ml": 150.0},
		Watchers: []WatcherConfig{{
			Resource: "button", Method: "Readings", Condition: map[string]interface{}{"pressed": true}, Poll: "5ms", Event: "estop",
		}},
		On: map[string]string{"estop": "faulted"},
		States: map[string]*StateConfig{
			"watching": {
				Task: &TaskConfig{
					Type: TaskTypeWaitFor, Resource: "glass_vision", Method: "Readings", Poll: "5ms",
					Condition: map[string]interface{}{"any": map[string]interface{}{
						"class_name": "empty_glass", "confidence": map[string]interface{}{"gt": 0.8},
					}},
				},
				On: map[string]string{"satisfied": "pouring"},
			},
			"pouring": {
				Task: &TaskConfig{
					Type: TaskTypeDoCommand, Resource: "pour_controller",
					Command: map[string]interface{}{"cmd": "pour", "ml": "$ctx.pour_ml"},
				},
				On: map[string]string{"poured": "checking"},
			},
			"checking": {
				Task: &TaskConfig{Type: TaskTypeCall, Resource: "button", Method: "Readings"},
				On:   map[string]string{"done": "idle"},
			},
			"idle":    {},
			"faulted": {On: map[string]string{"ack": "watching"}},
		},
	}
	validated, _, err := cfg.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, validated, test.ShouldHaveLength, 3)

	svc := newMachine(t, cfg, deps)
	ctx := context.Background()
	waitForPath(t, svc, "watching")

	glasses.Store(1)
	st := waitForPath(t, svc, "idle")
	test.That(t, commands, test.ShouldResemble, []map[string]interface{}{{"cmd": "pour", "ml": 150.0}})
	test.That(t, st.Context["ml"], test.ShouldEqual, 150.0)
	test.That(t, st.Context["outcome"], test.ShouldEqual, "poured")
	test.That(t, st.Context["pressed"], test.ShouldEqual, false)
	history, err := svc.GetHistory(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, history, test.ShouldHaveLength, 4)
	test.That(t, history[1].Trigger, test.ShouldEqual, task.OutcomeSatisfied)
	test.That(t, history[2].Trigger, test.ShouldEqual, "poured")
	test.That(t, history[3].Trigger, test.ShouldEqual, task.OutcomeDone)

	// The machine-level watcher fires on the rising edge of its condition, from any state.
	pressed.Store(true)
	waitForPath(t, svc, "faulted")
	glasses.Store(0)
	res, err := svc.SendEvent(ctx, "ack", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Path, test.ShouldResemble, []string{"watching"})
	// Still pressed: no second event until it is released and pressed again.
	time.Sleep(30 * time.Millisecond)
	st, err = svc.GetState(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.Path, test.ShouldResemble, []string{"watching"})
	pressed.Store(false)
	time.Sleep(15 * time.Millisecond)
	pressed.Store(true)
	waitForPath(t, svc, "faulted")
}

func TestConfigValidation(t *testing.T) {
	bad := []struct {
		name string
		cfg  *Config
		msg  string
	}{
		{"no initial", &Config{States: map[string]*StateConfig{"a": {}}}, "initial"},
		{"no states", &Config{Initial: "a"}, "states"},
		{"initial missing", &Config{Initial: "zzz", States: map[string]*StateConfig{"a": {}}}, "initial state"},
		{"bad target", &Config{Initial: "a", States: map[string]*StateConfig{"a": {On: map[string]string{"x": "nope"}}}}, "unknown state"},
		{"bad machine", &Config{Initial: "a", States: map[string]*StateConfig{"a": {Machine: "m"}}}, "unknown machine"},
		{"task on compound", &Config{Initial: "a", States: map[string]*StateConfig{
			"a": {Task: resourceTask("t"), Initial: "b", States: map[string]*StateConfig{"b": {}}},
		}}, "compound state cannot run a task"},
		{"bad duration", &Config{Initial: "a", States: map[string]*StateConfig{"a": {Timeout: "soon"}}}, "invalid duration"},
		{"bad task type", &Config{Initial: "a", States: map[string]*StateConfig{"a": {Task: &TaskConfig{Type: "magic"}}}}, "unknown task type"},
		{"call without method", &Config{
			Initial: "a", States: map[string]*StateConfig{"a": {Task: &TaskConfig{Type: "call", Resource: "r"}}},
		}, "method"},
		{"bad cancel timeout", &Config{Initial: "a", CancelTimeout: "x", States: map[string]*StateConfig{"a": {}}}, "invalid duration"},
		{"recursive machine", &Config{
			Initial: "a", States: map[string]*StateConfig{"a": {Machine: "m"}},
			Machines: map[string]*MachineConfig{"m": {Initial: "b", States: map[string]*StateConfig{"b": {Machine: "m"}}}},
		}, "references itself"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.cfg.Validate("")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.msg)
		})
	}

	// Targets resolve from siblings outward, and $history/$done are always allowed.
	cfg := &Config{
		Initial: "outer",
		States: map[string]*StateConfig{
			"outer": {Initial: "in", States: map[string]*StateConfig{
				"in": {On: map[string]string{"a": "other", "b": "$history", "c": "$done"}},
			}},
			"other": {},
		},
	}
	_, _, err := cfg.Validate("")
	test.That(t, err, test.ShouldBeNil)

	// Unknown dependencies fail at construction, not validation.
	_, err = NewBuiltIn(context.Background(), resource.Dependencies{}, resource.Config{
		Name: "sm", API: statemachine.API,
		ConvertedAttributes: &Config{Initial: "a", States: map[string]*StateConfig{"a": {Task: resourceTask("missing")}}},
	}, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldNotBeNil)
	_ = generic.API
}

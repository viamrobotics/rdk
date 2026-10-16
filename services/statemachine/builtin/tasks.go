package builtin

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
)

// runFunc is what a state runs: a task resource, or one of the built-in task
// types bound to a resource at construction.
type runFunc func(ctx context.Context, input map[string]interface{}, onProgress task.ProgressFunc) (task.Result, error)

func buildRunner(tc *TaskConfig, deps resource.Dependencies) (runFunc, error) {
	switch tc.Type {
	case TaskTypeResource:
		svc, err := task.FromProvider(deps, tc.Name)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, input map[string]interface{}, onProgress task.ProgressFunc) (task.Result, error) {
			if static, ok := resolveRefs(tc.Input, input).(map[string]interface{}); ok {
				for k, v := range static {
					input[k] = v
				}
			}
			return svc.Run(ctx, input, onProgress, nil)
		}, nil
	case TaskTypeDoCommand:
		res, err := findResource(deps, tc.Resource)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, input map[string]interface{}, _ task.ProgressFunc) (task.Result, error) {
			cmd, _ := resolveRefs(tc.Command, input).(map[string]interface{})
			resp, err := res.DoCommand(ctx, cmd)
			if err != nil {
				return task.Result{}, err
			}
			outcome := task.OutcomeDone
			if o, ok := resp["outcome"].(string); ok && o != "" {
				outcome = o
			}
			return task.Result{Outcome: outcome, Output: resp}, nil
		}, nil
	case TaskTypeCall:
		res, err := findResource(deps, tc.Resource)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, input map[string]interface{}, _ task.ProgressFunc) (task.Result, error) {
			out, err := invoke(ctx, res, tc.Method, resolveRefs(tc.Args, input))
			if err != nil {
				return task.Result{}, err
			}
			return task.Done(out), nil
		}, nil
	case TaskTypeWaitFor:
		res, err := findResource(deps, tc.Resource)
		if err != nil {
			return nil, err
		}
		poll, err := parseDuration(tc.Poll, defaultPoll)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, input map[string]interface{}, _ task.ProgressFunc) (task.Result, error) {
			args := resolveRefs(tc.Args, input)
			for {
				out, err := invoke(ctx, res, tc.Method, args)
				if err != nil {
					return task.Result{}, err
				}
				if Match(tc.Condition, out) {
					return task.Result{Outcome: task.OutcomeSatisfied, Output: out}, nil
				}
				select {
				case <-ctx.Done():
					return task.Result{}, ctx.Err()
				case <-time.After(poll):
				}
			}
		}, nil
	default:
		return nil, fmt.Errorf("unknown task type %q", tc.Type)
	}
}

// watcher polls a resource method while its owning state is active and
// emits an event when its condition starts to hold.
type watcher struct {
	cfg  WatcherConfig
	res  resource.Resource
	poll time.Duration
}

func buildWatcher(wc WatcherConfig, deps resource.Dependencies) (*watcher, error) {
	res, err := findResource(deps, wc.Resource)
	if err != nil {
		return nil, err
	}
	poll, err := parseDuration(wc.Poll, defaultPoll)
	if err != nil {
		return nil, err
	}
	return &watcher{cfg: wc, res: res, poll: poll}, nil
}

// run polls until ctx is cancelled, calling emit on each rising edge of the
// condition.
func (w *watcher) run(ctx context.Context, logger logging.Logger, emit func(event string, payload map[string]interface{})) {
	matched := false
	for {
		out, err := invoke(ctx, w.res, w.cfg.Method, w.cfg.Args)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return
			}
			logger.Warnw("watcher poll failed", "resource", w.cfg.Resource, "method", w.cfg.Method, "error", err)
		case Match(w.cfg.Condition, out):
			if !matched {
				emit(w.cfg.Event, out)
			}
			matched = true
		default:
			matched = false
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.poll):
		}
	}
}

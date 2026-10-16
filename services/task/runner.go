package task

import (
	"context"
	"sync"
	"time"

	"go.viam.com/rdk/resource"
)

// RunFunc is the single blocking method a task author writes. It reports
// progress through report (never nil), returns a Result when it is finished,
// and returns an error when it failed. When ctx is cancelled it must stop the
// hardware safely before returning.
type RunFunc func(ctx context.Context, input map[string]interface{}, report ProgressFunc) (Result, error)

// Runner implements Run and GetStatus for a task written as one RunFunc: it
// enforces one execution at a time, tracks the phase, start time, latest
// progress and last result. Embed it next to resource.Named to make a task
// resource, or use NewFromFunc.
type Runner struct {
	mu        sync.Mutex
	fn        RunFunc
	running   bool
	startedAt time.Time
	progress  *Progress
	last      *Result
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewRunner returns a Runner that executes fn.
func NewRunner(fn RunFunc) *Runner {
	return &Runner{fn: fn}
}

// Run executes the task once. It returns ErrAlreadyRunning if an execution is
// already in flight.
func (r *Runner) Run(
	ctx context.Context,
	input map[string]interface{},
	onProgress ProgressFunc,
	extra map[string]interface{},
) (Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return Result{}, ErrAlreadyRunning
	}
	done := make(chan struct{})
	r.running = true
	r.startedAt = time.Now()
	r.progress = nil
	r.cancel = cancel
	r.done = done
	r.mu.Unlock()

	report := func(p Progress) {
		r.mu.Lock()
		pCopy := p
		r.progress = &pCopy
		r.mu.Unlock()
		if onProgress != nil {
			onProgress(p)
		}
	}

	res, err := r.fn(ctx, input, report)
	if err == nil && res.Outcome == "" {
		res.Outcome = OutcomeDone
	}

	r.mu.Lock()
	r.running = false
	r.startedAt = time.Time{}
	r.progress = nil
	r.cancel = nil
	r.done = nil
	if err == nil {
		resCopy := res
		r.last = &resCopy
	} else {
		r.last = &Result{Outcome: OutcomeError, Output: map[string]interface{}{"error": err.Error()}}
	}
	r.mu.Unlock()
	close(done)
	return res, err
}

// GetStatus reports the runner's phase, last result, start time and progress.
func (r *Runner) GetStatus(ctx context.Context, extra map[string]interface{}) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Status{LastResult: r.last}
	if r.running {
		s.Phase = PhaseRunning
		s.StartedAt = r.startedAt
		s.Progress = r.progress
	}
	return s, nil
}

// Cancel cancels the in-flight execution, if any, and waits for it to return.
// Close implementations should call it so a task never outlives its resource.
func (r *Runner) Cancel() {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

// funcTask is a task resource built from a RunFunc.
type funcTask struct {
	resource.Named
	*Runner
}

// NewFromFunc returns a task resource with the given name that runs fn.
func NewFromFunc(name resource.Name, fn RunFunc) Service {
	return &funcTask{Named: name.AsNamed(), Runner: NewRunner(fn)}
}

// Close cancels any in-flight execution.
func (t *funcTask) Close(ctx context.Context) error {
	t.Cancel()
	return nil
}

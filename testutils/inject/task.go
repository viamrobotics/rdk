package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
)

// Task is an injected task service for testing.
type Task struct {
	task.Service
	name    resource.Name
	RunFunc func(
		ctx context.Context,
		input map[string]interface{},
		onProgress task.ProgressFunc,
		extra map[string]interface{},
	) (task.Result, error)
	GetStatusFunc func(ctx context.Context, extra map[string]interface{}) (task.Status, error)
	DoFunc        func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc     func(ctx context.Context) error
}

// NewTask returns a new injected task service.
func NewTask(name string) *Task {
	return &Task{name: task.Named(name)}
}

// Name returns the name of the injected task.
func (t *Task) Name() resource.Name {
	return t.name
}

// Run calls the injected RunFunc or the real version.
func (t *Task) Run(
	ctx context.Context,
	input map[string]interface{},
	onProgress task.ProgressFunc,
	extra map[string]interface{},
) (task.Result, error) {
	if t.RunFunc == nil {
		return t.Service.Run(ctx, input, onProgress, extra)
	}
	return t.RunFunc(ctx, input, onProgress, extra)
}

// GetStatus calls the injected GetStatusFunc or the real version.
func (t *Task) GetStatus(ctx context.Context, extra map[string]interface{}) (task.Status, error) {
	if t.GetStatusFunc == nil {
		return t.Service.GetStatus(ctx, extra)
	}
	return t.GetStatusFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (t *Task) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if t.DoFunc == nil {
		return t.Service.DoCommand(ctx, cmd)
	}
	return t.DoFunc(ctx, cmd)
}

// Status returns an empty status unless a Service is set.
func (t *Task) Status(ctx context.Context) (map[string]interface{}, error) {
	if t.Service != nil {
		return t.Service.Status(ctx)
	}
	return map[string]interface{}{}, nil
}

// Close calls the injected Close or the real version.
func (t *Task) Close(ctx context.Context) error {
	if t.CloseFunc == nil {
		if t.Service == nil {
			return nil
		}
		return t.Service.Close(ctx)
	}
	return t.CloseFunc(ctx)
}

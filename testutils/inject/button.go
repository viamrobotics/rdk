package inject

import (
	"context"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/resource"
)

// Button implements button.Button for testing.
type Button struct {
	button.Button

	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	PushFunc func(ctx context.Context, extra map[string]interface{}) error
	DoFunc   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Push calls PushFunc.
func (b *Button) Push(ctx context.Context, extra map[string]interface{}) error {
	if b.PushFunc == nil {
		return b.Button.Push(ctx, extra)
	}
	return b.PushFunc(ctx, extra)
}

// DoCommand calls DoFunc.
func (b *Button) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return b.Button.DoCommand(ctx, cmd)
	}
	return b.DoFunc(ctx, cmd)
}

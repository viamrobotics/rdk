package inject

import (
	"context"

	"go.viam.com/rdk/resource"
)

// Button implements button.Button for testing.
type Button struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	PushFunc func(ctx context.Context, extra map[string]interface{}) error
	DoFunc   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Push calls PushFunc.
func (b *Button) Push(ctx context.Context, extra map[string]interface{}) error {
	if b.PushFunc == nil {
		return nil
	}
	return b.PushFunc(ctx, extra)
}

// DoCommand calls DoFunc.
func (b *Button) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return nil, nil
	}
	return b.DoFunc(ctx, cmd)
}

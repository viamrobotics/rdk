package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/resource"
)

// Button implements button.Button for testing.
type Button struct {
	button.Button
	name       resource.Name
	PushFunc   func(ctx context.Context, extra map[string]interface{}) error
	DoFunc     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc func(ctx context.Context) (map[string]interface{}, error)
	CloseFunc  func(ctx context.Context) error
}

// NewButton returns a new injected button.
func NewButton(name string) *Button {
	return &Button{name: button.Named(name)}
}

// Name returns the name of the resource.
func (b *Button) Name() resource.Name {
	return b.name
}

// Push calls PushFunc.
func (b *Button) Push(ctx context.Context, extra map[string]interface{}) error {
	if b.PushFunc == nil {
		return errtrace.Wrap(b.Button.Push(ctx, extra))
	}
	return errtrace.Wrap(b.PushFunc(ctx, extra))
}

// DoCommand calls DoFunc.
func (b *Button) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return errtrace.Wrap2(b.Button.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(b.DoFunc(ctx, cmd))
}

// Close calls CloseFunc.
func (b *Button) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		return errtrace.Wrap(b.Button.Close(ctx))
	}
	return errtrace.Wrap(b.CloseFunc(ctx))
}

// Status calls the injected Status or the real version.
func (b *Button) Status(ctx context.Context) (map[string]interface{}, error) {
	if b.StatusFunc != nil {
		return errtrace.Wrap2(b.StatusFunc(ctx))
	}
	if b.Button != nil {
		return errtrace.Wrap2(b.Button.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

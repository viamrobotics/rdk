package inject

import (
	"context"

	"go.viam.com/rdk/components/encoder"
)

// Encoder is an injected encoder.
type Encoder struct {
	encoder.Encoder
	DoFunc            func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ResetPositionFunc func(ctx context.Context, offset float64, extra map[string]interface{}) error
	GetPositionFunc   func(ctx context.Context, extra map[string]interface{}) (float64, error)
}

// ResetPosition calls the injected Zero or the real version.
func (e *Encoder) ResetPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if e.ResetPositionFunc == nil {
		return e.Encoder.ResetPosition(ctx, offset, extra)
	}
	return e.ResetPositionFunc(ctx, offset, extra)
}

// GetPosition calls the injected Position or the real version.
func (e *Encoder) GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error) {
	if e.GetPositionFunc == nil {
		return e.Encoder.GetPosition(ctx, extra)
	}
	return e.GetPositionFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (e *Encoder) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if e.DoFunc == nil {
		return e.Encoder.DoCommand(ctx, cmd)
	}
	return e.DoFunc(ctx, cmd)
}

package inject

import (
	"context"

	"go.viam.com/rdk/components/encoder"
)

// Encoder is an injected encoder.
type Encoder struct {
	encoder.Encoder
	DoFunc            func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ResetPositionFunc func(ctx context.Context, extra map[string]interface{}) error
	GetPositionFunc   func(ctx context.Context,
		positionType *encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error)
	GetPropertiesFunc func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error)
}

// ResetPosition calls the injected Zero or the real version.
func (e *Encoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	if e.ResetPositionFunc == nil {
		return e.Encoder.ResetPosition(ctx, extra)
	}
	return e.ResetPositionFunc(ctx, extra)
}

// GetPosition calls the injected GetPosition or the real version.
func (e *Encoder) GetPosition(
	ctx context.Context,
	positionType *encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if e.GetPositionFunc == nil {
		return e.Encoder.GetPosition(ctx, positionType, extra)
	}
	return e.GetPositionFunc(ctx, positionType, extra)
}

// GetProperties calls the injected Properties or the real version.
func (e *Encoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	if e.GetPropertiesFunc == nil {
		return e.Encoder.GetProperties(ctx, extra)
	}
	return e.GetPropertiesFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (e *Encoder) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if e.DoFunc == nil {
		return e.Encoder.DoCommand(ctx, cmd)
	}
	return e.DoFunc(ctx, cmd)
}

package inject

import (
	"context"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/resource"
)

// Encoder is an injected encoder.
type Encoder struct {
	encoder.Encoder
	name              resource.Name
	DoFunc            func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ResetPositionFunc func(ctx context.Context, extra map[string]interface{}) error
	PositionFunc      func(ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error)
	PropertiesFunc func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error)
}

// NewEncoder returns a new injected Encoder.
func NewEncoder(name string) *Encoder {
	return &Encoder{name: encoder.Named(name)}
}

// Name returns the name of the resource.
func (e *Encoder) Name() resource.Name {
	return e.name
}

// ResetPosition calls the injected Zero or the real version.
func (e *Encoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	if e.ResetPositionFunc == nil {
		return e.Encoder.ResetPosition(ctx, extra)
	}
	return e.ResetPositionFunc(ctx, extra)
}

// Position calls the injected Position or the real version.
func (e *Encoder) Position(
	ctx context.Context,
	positionType encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if e.PositionFunc == nil {
		return e.Encoder.Position(ctx, positionType, extra)
	}
	return e.PositionFunc(ctx, positionType, extra)
}

// Properties calls the injected Properties or the real version.
func (e *Encoder) Properties(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
	if e.PropertiesFunc == nil {
		return e.Encoder.Properties(ctx, extra)
	}
	return e.PropertiesFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (e *Encoder) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if e.DoFunc == nil {
		return e.Encoder.DoCommand(ctx, cmd)
	}
	return e.DoFunc(ctx, cmd)
}

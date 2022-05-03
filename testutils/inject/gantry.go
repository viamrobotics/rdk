package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/gantry"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
)

// Gantry is an injected gantry.
type Gantry struct {
	gantry.Gantry
	DoFunc             func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetPositionFunc    func(ctx context.Context) ([]float64, error)
	MoveToPositionFunc func(ctx context.Context, positions []float64, worldState *commonpb.WorldState) error
	GetLengthsFunc     func(ctx context.Context) ([]float64, error)
	CloseFunc          func(ctx context.Context) error
	ModelFrameFunc     func() referenceframe.Model
}

// GetPosition calls the injected GetPosition or the real version.
func (a *Gantry) GetPosition(ctx context.Context) ([]float64, error) {
	if a.GetPositionFunc == nil {
		return a.Gantry.GetPosition(ctx)
	}
	return a.GetPositionFunc(ctx)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Gantry) MoveToPosition(ctx context.Context, positions []float64, worldState *commonpb.WorldState) error {
	if a.MoveToPositionFunc == nil {
		return a.Gantry.MoveToPosition(ctx, positions, worldState)
	}
	return a.MoveToPositionFunc(ctx, positions, worldState)
}

// GetLengths calls the injected GetLengths or the real version.
func (a *Gantry) GetLengths(ctx context.Context) ([]float64, error) {
	if a.GetLengthsFunc == nil {
		return a.Gantry.GetLengths(ctx)
	}
	return a.GetLengthsFunc(ctx)
}

// ModelFrame returns a Gantry ModelFrame.
func (a *Gantry) ModelFrame() referenceframe.Model {
	if a.ModelFrameFunc == nil {
		return a.Gantry.ModelFrame()
	}
	return a.ModelFrameFunc()
}

// Close calls the injected Close or the real version.
func (a *Gantry) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		return utils.TryClose(ctx, a.Gantry)
	}
	return a.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (a *Gantry) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return a.Gantry.Do(ctx, cmd)
	}
	return a.DoFunc(ctx, cmd)
}

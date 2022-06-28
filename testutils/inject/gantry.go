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
	gantry.LocalGantry
	DoFunc             func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetPositionFunc    func(ctx context.Context) ([]float64, error)
	MoveToPositionFunc func(ctx context.Context, positions []float64, worldState *commonpb.WorldState) error
	GetLengthsFunc     func(ctx context.Context) ([]float64, error)
	StopFunc           func(ctx context.Context) error
	IsMovingFunc       func() bool
	CloseFunc          func(ctx context.Context) error
	ModelFrameFunc     func() referenceframe.Model
}

// GetPosition calls the injected GetPosition or the real version.
func (g *Gantry) GetPosition(ctx context.Context) ([]float64, error) {
	if g.GetPositionFunc == nil {
		return g.LocalGantry.GetPosition(ctx)
	}
	return g.GetPositionFunc(ctx)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (g *Gantry) MoveToPosition(ctx context.Context, positions []float64, worldState *commonpb.WorldState) error {
	if g.MoveToPositionFunc == nil {
		return g.LocalGantry.MoveToPosition(ctx, positions, worldState)
	}
	return g.MoveToPositionFunc(ctx, positions, worldState)
}

// GetLengths calls the injected GetLengths or the real version.
func (g *Gantry) GetLengths(ctx context.Context) ([]float64, error) {
	if g.GetLengthsFunc == nil {
		return g.LocalGantry.GetLengths(ctx)
	}
	return g.GetLengthsFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (g *Gantry) Stop(ctx context.Context) error {
	if g.StopFunc == nil {
		return g.LocalGantry.Stop(ctx)
	}
	return g.StopFunc(ctx)
}

// IsMoving calls the injected IsMoving or the real version.
func (g *Gantry) IsMoving() bool {
	if g.IsMovingFunc == nil {
		return g.LocalGantry.IsMoving()
	}
	return g.IsMovingFunc()
}

// ModelFrame returns a Gantry ModelFrame.
func (g *Gantry) ModelFrame() referenceframe.Model {
	if g.ModelFrameFunc == nil {
		return g.LocalGantry.ModelFrame()
	}
	return g.ModelFrameFunc()
}

// Close calls the injected Close or the real version.
func (g *Gantry) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		return utils.TryClose(ctx, g.LocalGantry)
	}
	return g.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (g *Gantry) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.LocalGantry.Do(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

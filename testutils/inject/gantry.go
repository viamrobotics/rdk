package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/referenceframe"
)

// Gantry is an injected gantry.
type Gantry struct {
	gantry.LocalGantry
	DoFunc             func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	PositionFunc       func(ctx context.Context, extra map[string]interface{}) ([]float64, error)
	MoveToPositionFunc func(ctx context.Context, pos []float64, extra map[string]interface{}) error
	LengthsFunc        func(ctx context.Context, extra map[string]interface{}) ([]float64, error)
	StopFunc           func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc       func(context.Context) (bool, error)
	CloseFunc          func(ctx context.Context) error
	ModelFrameFunc     func() referenceframe.Model
}

// Position calls the injected Position or the real version.
func (g *Gantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	if g.PositionFunc == nil {
		return g.LocalGantry.Position(ctx, extra)
	}
	return g.PositionFunc(ctx, extra)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (g *Gantry) MoveToPosition(ctx context.Context, positions []float64, extra map[string]interface{}) error {
	if g.MoveToPositionFunc == nil {
		return g.LocalGantry.MoveToPosition(ctx, positions, extra)
	}
	return g.MoveToPositionFunc(ctx, positions, extra)
}

// Lengths calls the injected Lengths or the real version.
func (g *Gantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	if g.LengthsFunc == nil {
		return g.LocalGantry.Lengths(ctx, extra)
	}
	return g.LengthsFunc(ctx, extra)
}

// Stop calls the injected Stop or the real version.
func (g *Gantry) Stop(ctx context.Context, extra map[string]interface{}) error {
	if g.StopFunc == nil {
		return g.LocalGantry.Stop(ctx, extra)
	}
	return g.StopFunc(ctx, extra)
}

// IsMoving calls the injected IsMoving or the real version.
func (g *Gantry) IsMoving(ctx context.Context) (bool, error) {
	if g.IsMovingFunc == nil {
		return g.LocalGantry.IsMoving(ctx)
	}
	return g.IsMovingFunc(ctx)
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

// DoCommand calls the injected DoCommand or the real version.
func (g *Gantry) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.LocalGantry.DoCommand(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

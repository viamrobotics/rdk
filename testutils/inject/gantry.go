package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/gantry"
)

// Gantry is an injected gantry.
type Gantry struct {
	gantry.Gantry
	GetPositionFunc    func(ctx context.Context) ([]float64, error)
	MoveToPositionFunc func(ctx context.Context, positions []float64) error
	GetLengthsFunc     func(ctx context.Context) ([]float64, error)
	CloseFunc          func(ctx context.Context) error
}

// GetPosition calls the injected GetPosition or the real version.
func (a *Gantry) GetPosition(ctx context.Context) ([]float64, error) {
	if a.GetPositionFunc == nil {
		return a.Gantry.GetPosition(ctx)
	}
	return a.GetPositionFunc(ctx)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Gantry) MoveToPosition(ctx context.Context, positions []float64) error {
	if a.MoveToPositionFunc == nil {
		return a.Gantry.MoveToPosition(ctx, positions)
	}
	return a.MoveToPositionFunc(ctx, positions)
}

// GetLengths calls the injected GetLengths or the real version.
func (a *Gantry) GetLengths(ctx context.Context) ([]float64, error) {
	if a.GetLengthsFunc == nil {
		return a.Gantry.GetLengths(ctx)
	}
	return a.GetLengthsFunc(ctx)
}

// Close calls the injected Close or the real version.
func (a *Gantry) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		return utils.TryClose(ctx, a.Gantry)
	}
	return a.CloseFunc(ctx)
}

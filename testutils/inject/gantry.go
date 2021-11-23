package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/core/component/gantry"
)

// Gantry is an injected gantry.
type Gantry struct {
	gantry.Gantry
	CurrentPositionFunc func(ctx context.Context) ([]float64, error)
	MoveToPositionFunc  func(ctx context.Context, positions []float64) error
	LengthsFunc         func(ctx context.Context) ([]float64, error)
	CloseFunc           func() error
}

// CurrentPosition calls the injected CurrentPosition or the real version.
func (a *Gantry) CurrentPosition(ctx context.Context) ([]float64, error) {
	if a.CurrentPositionFunc == nil {
		return a.Gantry.CurrentPosition(ctx)
	}
	return a.CurrentPositionFunc(ctx)
}

// MoveToPosition calls the injected MoveToPosition or the real version.
func (a *Gantry) MoveToPosition(ctx context.Context, positions []float64) error {
	if a.MoveToPositionFunc == nil {
		return a.Gantry.MoveToPosition(ctx, positions)
	}
	return a.MoveToPositionFunc(ctx, positions)
}

// Lengths calls the injected Lengths or the real version.
func (a *Gantry) Lengths(ctx context.Context) ([]float64, error) {
	if a.LengthsFunc == nil {
		return a.Gantry.Lengths(ctx)
	}
	return a.LengthsFunc(ctx)
}

// Close calls the injected Close or the real version.
func (a *Gantry) Close() error {
	if a.CloseFunc == nil {
		return utils.TryClose(a.Gantry)
	}
	return a.CloseFunc()
}

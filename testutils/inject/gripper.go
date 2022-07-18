package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/gripper"
)

// Gripper is an injected gripper.
type Gripper struct {
	gripper.LocalGripper
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	OpenFunc     func(ctx context.Context) error
	GrabFunc     func(ctx context.Context) (bool, error)
	StopFunc     func(ctx context.Context) error
	IsMovingFunc func(context.Context) (bool, error)
	CloseFunc    func(ctx context.Context) error
}

// Open calls the injected Open or the real version.
func (g *Gripper) Open(ctx context.Context) error {
	if g.OpenFunc == nil {
		return g.LocalGripper.Open(ctx)
	}
	return g.OpenFunc(ctx)
}

// Grab calls the injected Grab or the real version.
func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	if g.GrabFunc == nil {
		return g.LocalGripper.Grab(ctx)
	}
	return g.GrabFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (g *Gripper) Stop(ctx context.Context) error {
	if g.StopFunc == nil {
		return g.LocalGripper.Stop(ctx)
	}
	return g.StopFunc(ctx)
}

// IsMoving calls the injected IsMoving or the real version.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	if g.IsMovingFunc == nil {
		return g.LocalGripper.IsMoving(ctx)
	}
	return g.IsMovingFunc(ctx)
}

// Close calls the injected Close or the real version.
func (g *Gripper) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		return utils.TryClose(ctx, g.LocalGripper)
	}
	return g.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (g *Gripper) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.LocalGripper.Do(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

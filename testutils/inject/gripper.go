package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/resource"
)

// Gripper is an injected gripper.
type Gripper struct {
	gripper.Gripper
	name         resource.Name
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	OpenFunc     func(ctx context.Context, extra map[string]interface{}) error
	GrabFunc     func(ctx context.Context, extra map[string]interface{}) (bool, error)
	StopFunc     func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc func(context.Context) (bool, error)
	CloseFunc    func(ctx context.Context) error
}

// NewGripper returns a new injected gripper.
func NewGripper(name string) *Gripper {
	return &Gripper{name: gripper.Named(name)}
}

// Name returns the name of the resource.
func (g *Gripper) Name() resource.Name {
	return g.name
}

// Open calls the injected Open or the real version.
func (g *Gripper) Open(ctx context.Context, extra map[string]interface{}) error {
	if g.OpenFunc == nil {
		return g.Gripper.Open(ctx, extra)
	}
	return g.OpenFunc(ctx, extra)
}

// Grab calls the injected Grab or the real version.
func (g *Gripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	if g.GrabFunc == nil {
		return g.Gripper.Grab(ctx, extra)
	}
	return g.GrabFunc(ctx, extra)
}

// Stop calls the injected Stop or the real version.
func (g *Gripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	if g.StopFunc == nil {
		return g.Gripper.Stop(ctx, extra)
	}
	return g.StopFunc(ctx, extra)
}

// IsMoving calls the injected IsMoving or the real version.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	if g.IsMovingFunc == nil {
		return g.Gripper.IsMoving(ctx)
	}
	return g.IsMovingFunc(ctx)
}

// Close calls the injected Close or the real version.
func (g *Gripper) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		return utils.TryClose(ctx, g.Gripper)
	}
	return g.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (g *Gripper) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.Gripper.DoCommand(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

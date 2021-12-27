package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/gripper"
)

// Gripper is an injected gripper.
type Gripper struct {
	gripper.Gripper
	OpenFunc  func(ctx context.Context) error
	GrabFunc  func(ctx context.Context) (bool, error)
	CloseFunc func(ctx context.Context) error
}

// Open calls the injected Open or the real version.
func (g *Gripper) Open(ctx context.Context) error {
	if g.OpenFunc == nil {
		return g.Gripper.Open(ctx)
	}
	return g.OpenFunc(ctx)
}

// Grab calls the injected Grab or the real version.
func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	if g.GrabFunc == nil {
		return g.Gripper.Grab(ctx)
	}
	return g.GrabFunc(ctx)
}

// Close calls the injected Close or the real version.
func (g *Gripper) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		return utils.TryClose(ctx, g.Gripper)
	}
	return g.CloseFunc(ctx)
}

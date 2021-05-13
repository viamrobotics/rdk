package inject

import (
	"context"

	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/utils"
)

type Gripper struct {
	gripper.Gripper
	OpenFunc  func(ctx context.Context) error
	GrabFunc  func(ctx context.Context) (bool, error)
	CloseFunc func() error
}

func (g *Gripper) Open(ctx context.Context) error {
	if g.OpenFunc == nil {
		return g.Gripper.Open(ctx)
	}
	return g.OpenFunc(ctx)
}

func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	if g.GrabFunc == nil {
		return g.Gripper.Grab(ctx)
	}
	return g.GrabFunc(ctx)
}

func (g *Gripper) Close() error {
	if g.CloseFunc == nil {
		return utils.TryClose(g.Gripper)
	}
	return g.CloseFunc()
}

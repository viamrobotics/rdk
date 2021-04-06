package inject

import (
	"context"

	"go.viam.com/robotcore/api"
)

type Gripper struct {
	api.Gripper
	OpenFunc  func(ctx context.Context) error
	GrabFunc  func(ctx context.Context) (bool, error)
	CloseFunc func(ctx context.Context) error
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

func (g *Gripper) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		return g.Gripper.Close(ctx)
	}
	return g.CloseFunc(ctx)
}

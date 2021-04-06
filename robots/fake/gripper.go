package fake

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterGripper(ModelName, func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (api.Gripper, error) {
		return &Gripper{}, nil
	})
}

type Gripper struct {
}

func (g *Gripper) Open(ctx context.Context) error {
	return nil
}

func (g *Gripper) Close(ctx context.Context) error {
	return nil
}

func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	return false, nil
}

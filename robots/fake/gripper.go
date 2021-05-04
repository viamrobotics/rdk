package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterGripper(ModelName, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (api.Gripper, error) {
		return &Gripper{Name: config.Name}, nil
	})
}

type Gripper struct {
	Name string
}

func (g *Gripper) Open(ctx context.Context) error {
	return nil
}

func (g *Gripper) Close() error {
	return nil
}

func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	return false, nil
}

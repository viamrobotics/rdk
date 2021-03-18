package fake

import (
	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterGripper(ModelName, func(r api.Robot, config api.Component) (api.Gripper, error) {
		return &Gripper{}, nil
	})
}

type Gripper struct {
}

func (g *Gripper) Open() error {
	return nil
}

func (g *Gripper) Close() error {
	return nil
}

func (g *Gripper) Grab() (bool, error) {
	return false, nil
}

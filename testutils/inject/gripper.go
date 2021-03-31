package inject

import (
	"go.viam.com/robotcore/api"
)

type Gripper struct {
	api.Gripper
	OpenFunc  func() error
	GrabFunc  func() (bool, error)
	CloseFunc func() error
}

func (g *Gripper) Open() error {
	if g.OpenFunc == nil {
		return g.Gripper.Open()
	}
	return g.OpenFunc()
}

func (g *Gripper) Grab() (bool, error) {
	if g.GrabFunc == nil {
		return g.Gripper.Grab()
	}
	return g.GrabFunc()
}

func (g *Gripper) Close() error {
	if g.CloseFunc == nil {
		return g.Gripper.Close()
	}
	return g.CloseFunc()
}

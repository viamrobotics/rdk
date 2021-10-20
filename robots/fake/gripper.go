package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/golang/geo/r3"
)

func init() {
	registry.RegisterGripper(ModelName, registry.Gripper{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
			frame, err := referenceframe.FrameFromPoint(config.Name, r3.Vector{0, 0, 200})
			if err != nil {
				return nil, err
			}
			return &Gripper{Name: config.Name, frame: frame, frameconfig: config.Frame}, nil
		},
		Frame: func(name string) (referenceframe.Frame, error) {
			return referenceframe.FrameFromPoint(name, r3.Vector{0, 0, 200})
		},
	})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	Name        string
	frame       referenceframe.Frame
	frameconfig *config.Frame
}

// Frame returns the intrinsic frame of the gripper
func (g *Gripper) Frame() referenceframe.Frame {
	return g.frame
}

// FrameSystemLink returns all the information necessary for including the gripper in a FrameSystem
func (g *Gripper) FrameSystemLink() (*config.Frame, referenceframe.Frame) {
	return g.frameconfig, g.frame
}

// Open does nothing.
func (g *Gripper) Open(ctx context.Context) error {
	return nil
}

// Close does nothing.
func (g *Gripper) Close() error {
	return nil
}

// Grab does nothing.
func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	return false, nil
}

package robot

import (
	"fmt"

	"github.com/echolabsinc/robotcore/arm"
	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/gripper"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
)

type Robot struct {
	Arms     []*arm.URArm       // TODO(erh): use interface
	Grippers []*gripper.Gripper // TODO(erh): use interface
	Cameras  []vision.MatSource
	Bases    []base.Base

	armComponents     []Component
	gripperComponents []Component
	cameraComponents  []Component
	baseComponents    []Component
}

func (r *Robot) ArmByName(name string) *arm.URArm {
	for i, c := range r.armComponents {
		if c.Name == name {
			return r.Arms[i]
		}
	}
	return nil
}

func (r *Robot) GripperByName(name string) *gripper.Gripper {
	for i, c := range r.gripperComponents {
		if c.Name == name {
			return r.Grippers[i]
		}
	}
	return nil
}

func (r *Robot) CameraByName(name string) vision.MatSource {
	for i, c := range r.cameraComponents {
		if c.Name == name {
			return r.Cameras[i]
		}
	}
	return nil
}

func (r *Robot) AddArm(a *arm.URArm, c Component) {
	r.Arms = append(r.Arms, a)
	r.armComponents = append(r.armComponents, c)
}

func (r *Robot) AddGripper(g *gripper.Gripper, c Component) {
	r.Grippers = append(r.Grippers, g)
	r.gripperComponents = append(r.gripperComponents, c)
}
func (r *Robot) AddCamera(camera vision.MatSource, c Component) {
	r.Cameras = append(r.Cameras, camera)
	r.cameraComponents = append(r.cameraComponents, c)
}
func (r *Robot) AddBase(b base.Base, c Component) {
	r.Bases = append(r.Bases, b)
	r.baseComponents = append(r.baseComponents, c)
}

func (r *Robot) Close() {
	for _, x := range r.Arms {
		x.Close()
	}

	for _, x := range r.Grippers {
		x.Close()
	}

	for _, x := range r.Cameras {
		x.Close()
	}

	for _, x := range r.Bases {
		x.Close()
	}

}

func NewBlankRobot() *Robot {
	return &Robot{}
}

func NewRobot(cfg Config) (*Robot, error) {
	r := &Robot{}
	logger := cfg.Logger
	if logger == nil {
		logger = golog.Global
	}

	for _, c := range cfg.Components {
		switch c.Type {
		case Arm:
			a, err := newArm(c)
			if err != nil {
				return nil, err
			}
			r.AddArm(a, c)
		case Gripper:
			g, err := newGripper(c, logger)
			if err != nil {
				return nil, err
			}
			r.AddGripper(g, c)
		case Camera:
			camera, err := newCamera(c)
			if err != nil {
				return nil, err
			}
			r.AddCamera(camera, c)
		default:
			return nil, fmt.Errorf("unknown component type: %v", c.Type)
		}
	}

	return r, nil
}

func newArm(config Component) (*arm.URArm, error) {
	switch config.Model {
	case "ur":
		return arm.URArmConnect(config.Host)
	default:
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}
}

func newGripper(config Component, logger golog.Logger) (*gripper.Gripper, error) {
	switch config.Model {
	case "robotiq":
		return gripper.NewGripper(config.Host, logger)
	default:
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
}

func newCamera(config Component) (vision.MatSource, error) {
	switch config.Model {
	case "eliot":
		golog.Global.Warn("using 'eliot' as a camera source, should switch to intel")
		return vision.NewIntelServerSource(config.Host, config.Port, config.Attributes), nil
	case "intel":
		return vision.NewIntelServerSource(config.Host, config.Port, config.Attributes), nil

	case "url":
		if len(config.Attributes) == 0 {
			return nil, fmt.Errorf("camera 'url' needs a color attribute (and a depth if you have it)")
		}
		return &vision.HTTPSource{config.Attributes["color"], config.Attributes["depth"]}, nil

	default:
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
}

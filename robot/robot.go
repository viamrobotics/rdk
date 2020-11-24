package robot

import (
	"fmt"

	"github.com/echolabsinc/robotcore/arm"
	"github.com/echolabsinc/robotcore/gripper"
	"github.com/echolabsinc/robotcore/vision"
)

type Robot struct {
	Arms     []*arm.URArm       // TODO: use interface
	Grippers []*gripper.Gripper // TODO: use interface
	Cameras  []vision.MatSource
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

}

func NewRobot(cfg Config) (*Robot, error) {
	r := &Robot{}

	for _, c := range cfg.Components {
		switch c.Type {
		case Arm:
			a, err := newArm(c)
			if err != nil {
				return nil, err
			}
			r.Arms = append(r.Arms, a)
		case Gripper:
			g, err := newGripper(c)
			if err != nil {
				return nil, err
			}
			r.Grippers = append(r.Grippers, g)
		case Camera:
			camera, err := newCamera(c)
			if err != nil {
				return nil, err
			}
			r.Cameras = append(r.Cameras, camera)
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

func newGripper(config Component) (*gripper.Gripper, error) {
	switch config.Model {
	case "robotiq":
		return gripper.NewGripper(config.Host)
	default:
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
}

func newCamera(config Component) (vision.MatSource, error) {
	switch config.Model {
	case "eliot":
		return vision.NewHttpSourceIntelEliot(fmt.Sprintf("%s:%d", config.Host, config.Port)), nil
	default:
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
}

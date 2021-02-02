package robot

import (
	"fmt"
	"time"

	"github.com/viamrobotics/robotcore/arm"
	"github.com/viamrobotics/robotcore/base"
	"github.com/viamrobotics/robotcore/gripper"
	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/robots/fake"
	"github.com/viamrobotics/robotcore/robots/hellorobot"
	"github.com/viamrobotics/robotcore/vision"

	"github.com/edaniels/golog"
)

type Robot struct {
	Arms         []arm.Arm
	Grippers     []gripper.Gripper
	Cameras      []vision.ImageDepthSource
	LidarDevices []lidar.Device
	Bases        []base.Base

	robotSingletons   map[string]interface{}
	armComponents     []Component
	gripperComponents []Component
	cameraComponents  []Component
	lidarComponents   []Component
	baseComponents    []Component
}

func (r *Robot) ArmByName(name string) arm.Arm {
	for i, c := range r.armComponents {
		if c.Name == name {
			return r.Arms[i]
		}
	}
	return nil
}

func (r *Robot) GripperByName(name string) gripper.Gripper {
	for i, c := range r.gripperComponents {
		if c.Name == name {
			return r.Grippers[i]
		}
	}
	return nil
}

func (r *Robot) CameraByName(name string) vision.ImageDepthSource {
	for i, c := range r.cameraComponents {
		if c.Name == name {
			return r.Cameras[i]
		}
	}
	return nil
}

func (r *Robot) LidarDeviceByName(name string) lidar.Device {
	for i, c := range r.lidarComponents {
		if c.Name == name {
			return r.LidarDevices[i]
		}
	}
	return nil
}

func (r *Robot) AddArm(a arm.Arm, c Component) {
	r.Arms = append(r.Arms, a)
	r.armComponents = append(r.armComponents, c)
}

func (r *Robot) AddGripper(g gripper.Gripper, c Component) {
	r.Grippers = append(r.Grippers, g)
	r.gripperComponents = append(r.gripperComponents, c)
}
func (r *Robot) AddCamera(camera vision.ImageDepthSource, c Component) {
	r.Cameras = append(r.Cameras, camera)
	r.cameraComponents = append(r.cameraComponents, c)
}
func (r *Robot) AddLidar(device lidar.Device, c Component) {
	r.LidarDevices = append(r.LidarDevices, device)
	r.lidarComponents = append(r.lidarComponents, c)
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

	for _, x := range r.LidarDevices {
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
	r := &Robot{robotSingletons: map[string]interface{}{}}
	logger := cfg.Logger
	if logger == nil {
		logger = golog.Global
	}

	for _, c := range cfg.Components {
		switch c.Type {
		case ComponentTypeBase:
			b, err := r.newBase(c)
			if err != nil {
				return nil, err
			}
			r.AddBase(b, c)
		case ComponentTypeArm:
			a, err := r.newArm(c)
			if err != nil {
				return nil, err
			}
			r.AddArm(a, c)
		case ComponentTypeGripper:
			g, err := r.newGripper(c, logger)
			if err != nil {
				return nil, err
			}
			r.AddGripper(g, c)
		case ComponentTypeCamera:
			camera, err := r.newCamera(c)
			if err != nil {
				return nil, err
			}
			r.AddCamera(camera, c)
		case ComponentTypeLidar:
			return nil, fmt.Errorf("TODO(erd): %v not yet supported via configuration", c.Type)
		default:
			return nil, fmt.Errorf("unknown component type: %v", c.Type)
		}
	}

	return r, nil
}

// TODO(erd): prefer registration pattern
func (r *Robot) getRobotSingleton(name string) interface{} {
	if root, ok := r.robotSingletons[name]; ok {
		return root
	}
	switch name {
	case hellorobot.ModelName:
		r.robotSingletons[name] = hellorobot.New()
	default:
		panic(fmt.Errorf("do not know how to get root for %q", name))
	}
	return r.robotSingletons[name]
}

// TODO(erd): prefer registration pattern
func (r *Robot) newBase(config Component) (base.Base, error) {
	switch config.Model {
	case fake.ModelName:
		return &fake.Base{}, nil
	case hellorobot.ModelName:
		return r.getRobotSingleton(config.Model).(*hellorobot.Robot).Base(), nil
	default:
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newArm(config Component) (arm.Arm, error) {
	switch config.Model {
	case "ur":
		return arm.URArmConnect(config.Host)
	case "eva":
		return arm.NewEva(config.Host, config.Attributes)
	case fake.ModelName:
		return &fake.Arm{}, nil
	case hellorobot.ModelName:
		return r.getRobotSingleton(config.Model).(*hellorobot.Robot).Arm(), nil
	default:
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newGripper(config Component, logger golog.Logger) (gripper.Gripper, error) {
	switch config.Model {
	case "robotiq":
		return gripper.NewRobotiqGripper(config.Host, logger)
	case "serial":

		port, err := ConnectArduinoSerial("A")
		if err != nil {
			return nil, err
		}

		time.Sleep(1000 * time.Millisecond) // wait for startup?

		return gripper.NewSerialGripper(port)
	case fake.ModelName:
		return &fake.Gripper{}, nil
	default:
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newCamera(config Component) (vision.ImageDepthSource, error) {
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

	case "file":
		return &vision.FileSource{config.Attributes["color"], config.Attributes["depth"]}, nil

	default:
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
}

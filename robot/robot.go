package robot

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/viamrobotics/rplidar"
	"go.viam.com/robotcore/arm"
	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/robots/hellorobot"
	"go.viam.com/robotcore/robots/minirover2"
	"go.viam.com/robotcore/serial"
	"go.viam.com/robotcore/vision"

	_ "github.com/viamrobotics/rplidar/ws" // registration

	"github.com/edaniels/golog"
)

type Robot struct {
	Boards []board.Board

	Arms         []arm.Arm
	Grippers     []gripper.Gripper
	Cameras      []vision.ImageDepthSource
	LidarDevices []lidar.Device
	Bases        []base.Device
	providers    []interface{}

	armComponents      []Component
	gripperComponents  []Component
	cameraComponents   []Component
	lidarComponents    []Component
	baseComponents     []Component
	providerComponents []Component
}

// theRobot.ComponentFor( theRobot.Arms[0] )
func (r *Robot) ComponentFor(theThing interface{}) *Component {

	for idx, a := range r.Arms {
		if theThing == a {
			return &r.armComponents[idx]
		}
	}

	for idx, g := range r.Grippers {
		if theThing == g {
			return &r.gripperComponents[idx]
		}
	}

	for idx, c := range r.Cameras {
		if theThing == c {
			return &r.cameraComponents[idx]
		}
	}

	for idx, l := range r.LidarDevices {
		if theThing == l {
			return &r.lidarComponents[idx]
		}
	}

	for idx, b := range r.Bases {
		if theThing == b {
			return &r.baseComponents[idx]
		}
	}

	return nil
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

func (r *Robot) providerByModel(model string) (interface{}, error) {
	for i, c := range r.providerComponents {
		if c.Model == model {
			return r.providers[i], nil
		}
	}
	return nil, fmt.Errorf("no provider for model %q", model)
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
func (r *Robot) AddBase(b base.Device, c Component) {
	r.Bases = append(r.Bases, b)
	r.baseComponents = append(r.baseComponents, c)
}
func (r *Robot) AddProvider(p interface{}, c Component) {
	r.providers = append(r.providers, p)
	r.providerComponents = append(r.providerComponents, c)
}

func (r *Robot) Close(ctx context.Context) error {
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
		if err := x.Close(ctx); err != nil {
			golog.Global.Error("error closing lidar device", "error", err)
		}
	}

	for _, x := range r.Bases {
		x.Close()
	}

	return nil
}

func NewBlankRobot() *Robot {
	return &Robot{}
}

func NewRobot(ctx context.Context, cfg Config) (*Robot, error) {
	r := &Robot{}
	logger := cfg.Logger
	if logger == nil {
		logger = golog.Global
	}

	for _, c := range cfg.Boards {
		b, err := board.NewBoard(c)
		if err != nil {
			return nil, err
		}
		r.Boards = append(r.Boards, b)
	}

	for _, c := range cfg.Components {
		switch c.Type {
		case ComponentTypeProvider:
			p, err := r.newProvider(c)
			if err != nil {
				return nil, err
			}
			r.AddProvider(p, c)
		}
	}

	for _, c := range cfg.Components {
		switch c.Type {
		case ComponentTypeProvider:
			// hanlded above
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
			lidarDevice, err := r.newLidar(ctx, c)
			if err != nil {
				return nil, err
			}
			r.AddLidar(lidarDevice, c)
		default:
			return nil, fmt.Errorf("unknown component type: %v", c.Type)
		}
	}

	return r, nil
}

// TODO(erd): prefer registration pattern
func (r *Robot) newProvider(config Component) (interface{}, error) {
	switch config.Model {
	case hellorobot.ModelName:
		return hellorobot.New(), nil
	case minirover2.ModelName:
		if len(r.Boards) != 1 {
			return nil, fmt.Errorf("minirover2 needs exactly 1 baord right now")
		}
		rover, err := minirover2.NewRover(r.Boards[0])
		if err != nil {
			return nil, err
		}
		return rover, nil
	default:
		return nil, fmt.Errorf("unknown provider model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newBase(config Component) (base.Device, error) {
	switch config.Model {
	case fake.ModelName:
		return &fake.Base{}, nil
	case hellorobot.ModelName:
		t, err := r.providerByModel(hellorobot.ModelName)
		if err != nil {
			return nil, err
		}
		return t.(*hellorobot.Robot).Base(), nil
	case minirover2.ModelName:
		t, err := r.providerByModel(minirover2.ModelName)
		if err != nil {
			return nil, err
		}
		return t.(base.Device), nil
	default:
		return nil, fmt.Errorf("unknown base model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newArm(config Component) (arm.Arm, error) {
	switch config.Model {
	case "ur":
		return arm.URArmConnect(config.Host)
	case "eva":
		return arm.NewEva(config.Host, config.Attributes)
	case "wx250s":
		mutex := &sync.Mutex{}
		for _, grip := range r.Grippers {
			switch sGrip := grip.(type) {
			case *gripper.Wx250s:
				mutex = sGrip.GetMoveLock()
			}
		}
		return arm.NewWx250s(config.Attributes, mutex)
	case fake.ModelName:
		return &fake.Arm{}, nil
	case hellorobot.ModelName:
		t, err := r.providerByModel(hellorobot.ModelName)
		if err != nil {
			return nil, err
		}
		return t.(*hellorobot.Robot).Arm(), nil
	default:
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newGripper(config Component, logger golog.Logger) (gripper.Gripper, error) {
	switch config.Model {
	case "robotiq":
		return gripper.NewRobotiqGripper(config.Host, logger)
	case "wx250s":
		mutex := &sync.Mutex{}
		for _, thisArm := range r.Arms {
			switch sArm := thisArm.(type) {
			case *arm.Wx250s:
				mutex = sArm.GetMoveLock()
			}
		}
		return gripper.NewWx250s(config.Attributes, mutex)
	case "serial":

		devices, err := serial.SearchDevices(serial.SearchFilter{Type: serial.DeviceTypeArduino})
		if err != nil {
			return nil, err
		}
		if len(devices) == 0 {
			return nil, errors.New("no applicable serial devices found for gripper")
		}
		device, err := serial.OpenDevice(devices[0].Path)
		if err != nil {
			return nil, err
		}

		time.Sleep(1000 * time.Millisecond) // wait for startup?

		return gripper.NewSerialGripper(device)
	case "viam":
		if len(r.Boards) != 1 {
			return nil, fmt.Errorf("viam gripper requires exactly 1 board")
		}
		return gripper.NewViamGripper(r.Boards[0])
	case fake.ModelName:
		return &fake.Gripper{}, nil
	default:
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newCamera(config Component) (vision.ImageDepthSource, error) {
	src, err := r.newCameraLL(config)
	if err != nil {
		return nil, err
	}

	if config.Attributes["rotate"] == "true" {
		src = &vision.RotateImageDepthSource{src}
	}

	return src, nil
}

func (r *Robot) newCameraLL(config Component) (vision.ImageDepthSource, error) {
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

	case "webcam":
		return vision.NewWebcamSource(config.Attributes)

	default:
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newLidar(ctx context.Context, config Component) (lidar.Device, error) {
	switch config.Model {
	case rplidar.ModelName:
		return lidar.CreateDevice(ctx, lidar.DeviceDescription{
			Type: rplidar.DeviceType,
			Host: config.Host,
			Port: config.Port,
		})
	case fake.ModelName:
		return fake.NewLidar(), nil
	default:
		return nil, fmt.Errorf("unknown lidar model: %s", config.Model)
	}
}

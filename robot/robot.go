package robot

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/arm"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/robots/hellorobot"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type Robot struct {
	Boards []board.Board

	Arms         []api.Arm
	Grippers     []api.Gripper
	Cameras      []gostream.ImageSource
	LidarDevices []lidar.Device
	Bases        []api.Base
	providers    []api.Provider

	boardComponents    []board.Config
	armComponents      []api.Component
	gripperComponents  []api.Component
	cameraComponents   []api.Component
	lidarComponents    []api.Component
	baseComponents     []api.Component
	providerComponents []api.Component
}

// theRobot.ComponentFor( theRobot.Arms[0] )
func (r *Robot) ComponentFor(theThing interface{}) *api.Component {

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

func (r *Robot) BoardByName(name string) board.Board {
	for i, c := range r.boardComponents {
		if c.Name == name {
			return r.Boards[i]
		}
	}
	return nil
}

func (r *Robot) ArmByName(name string) api.Arm {
	for i, c := range r.armComponents {
		if c.Name == name {
			return r.Arms[i]
		}
	}
	return nil
}

func (r *Robot) GripperByName(name string) api.Gripper {
	for i, c := range r.gripperComponents {
		if c.Name == name {
			return r.Grippers[i]
		}
	}
	return nil
}

func (r *Robot) CameraByName(name string) gostream.ImageSource {
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

func (r *Robot) AddBoard(b board.Board, c board.Config) {
	r.Boards = append(r.Boards, b)
	r.boardComponents = append(r.boardComponents, c)
}

func (r *Robot) AddArm(a api.Arm, c api.Component) {
	r.Arms = append(r.Arms, a)
	r.armComponents = append(r.armComponents, c)
}

func (r *Robot) AddGripper(g api.Gripper, c api.Component) {
	r.Grippers = append(r.Grippers, g)
	r.gripperComponents = append(r.gripperComponents, c)
}
func (r *Robot) AddCamera(camera gostream.ImageSource, c api.Component) {
	r.Cameras = append(r.Cameras, camera)
	r.cameraComponents = append(r.cameraComponents, c)
}
func (r *Robot) AddLidar(device lidar.Device, c api.Component) {
	r.LidarDevices = append(r.LidarDevices, device)
	r.lidarComponents = append(r.lidarComponents, c)
}
func (r *Robot) AddBase(b api.Base, c api.Component) {
	r.Bases = append(r.Bases, b)
	r.baseComponents = append(r.baseComponents, c)
}
func (r *Robot) AddProvider(p api.Provider, c api.Component) {
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
		if err := x.Close(ctx); err != nil {
			golog.Global.Error("error closing base device", "error", err)
		}
	}

	return nil
}

func NewBlankRobot() *Robot {
	return &Robot{}
}

func NewRobot(ctx context.Context, cfg api.Config) (*Robot, error) {
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
		r.AddBoard(b, c)
	}

	for _, c := range cfg.Components {
		switch c.Type {
		case api.ComponentTypeProvider:
			p, err := r.newProvider(c)
			if err != nil {
				return nil, err
			}
			r.AddProvider(p, c)
		}
	}

	for _, c := range cfg.Components {
		switch c.Type {
		case api.ComponentTypeProvider:
			// hanlded above
		case api.ComponentTypeBase:
			b, err := r.newBase(c)
			if err != nil {
				return nil, err
			}
			r.AddBase(b, c)
		case api.ComponentTypeArm:
			a, err := r.newArm(c)
			if err != nil {
				return nil, err
			}
			r.AddArm(a, c)
		case api.ComponentTypeGripper:
			g, err := r.newGripper(c, logger)
			if err != nil {
				return nil, err
			}
			r.AddGripper(g, c)
		case api.ComponentTypeCamera:
			camera, err := r.newCamera(c)
			if err != nil {
				return nil, err
			}
			r.AddCamera(camera, c)
		case api.ComponentTypeLidar:
			lidarDevice, err := r.newLidar(ctx, c)
			if err != nil {
				return nil, err
			}
			r.AddLidar(lidarDevice, c)
		default:
			return nil, fmt.Errorf("unknown component type: %v", c.Type)
		}
	}

	for _, p := range r.providers {
		err := p.Ready(r)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// TODO(erd): prefer registration pattern
func (r *Robot) newProvider(config api.Component) (api.Provider, error) {
	switch config.Model {
	case hellorobot.ModelName:
		return hellorobot.New()
	default:
		return nil, fmt.Errorf("unknown provider model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newBase(config api.Component) (api.Base, error) {
	switch config.Model {
	case fake.ModelName:
		return &fake.Base{}, nil
	case hellorobot.ModelName:
		t, err := r.providerByModel(hellorobot.ModelName)
		if err != nil {
			return nil, err
		}
		return t.(*hellorobot.Robot).Base()
	default:
		return nil, fmt.Errorf("unknown base model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newArm(config api.Component) (api.Arm, error) {
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
		return t.(*hellorobot.Robot).Arm()
	default:
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newGripper(config api.Component, logger golog.Logger) (api.Gripper, error) {
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
func (r *Robot) newCamera(config api.Component) (gostream.ImageSource, error) {
	src, err := r.newCameraLL(config)
	if err != nil {
		return nil, err
	}

	if config.Attributes["rotate"] == "true" {
		src = &rimage.RotateImageDepthSource{src}
	}

	return src, nil
}

func (r *Robot) newCameraLL(config api.Component) (gostream.ImageSource, error) {
	switch config.Model {
	case "eliot":
		golog.Global.Warn("using 'eliot' as a camera source, should switch to intel")
		return rimage.NewIntelServerSource(config.Host, config.Port, config.Attributes), nil
	case "intel":
		return rimage.NewIntelServerSource(config.Host, config.Port, config.Attributes), nil

	case "url":
		if len(config.Attributes) == 0 {
			return nil, fmt.Errorf("camera 'url' needs a color attribute (and a depth if you have it)")
		}
		return &rimage.HTTPSource{config.Attributes.GetString("color"), config.Attributes.GetString("depth")}, nil

	case "file":
		return &rimage.FileSource{config.Attributes.GetString("color"), config.Attributes.GetString("depth")}, nil

	case "webcam":
		return rimage.NewWebcamSource(config.Attributes)

	case "depthComposed":
		return newDepthComposed(r, config)

	case "overlay":
		return newOverlay(r, config)

	default:
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
}

// TODO(erd): prefer registration pattern
func (r *Robot) newLidar(ctx context.Context, config api.Component) (lidar.Device, error) {
	switch config.Model {
	case lidar.ModelNameWS:
		return lidar.CreateDevice(ctx, lidar.DeviceDescription{
			Type: lidar.DeviceTypeWS,
			Host: config.Host,
			Port: config.Port,
		})
	case fake.ModelName:
		return fake.NewLidar(), nil
	default:
		return nil, fmt.Errorf("unknown lidar model: %s", config.Model)
	}
}

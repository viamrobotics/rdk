package robot

import (
	"context"
	"fmt"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"

	// registration
	_ "go.viam.com/robotcore/lidar/client"
	_ "go.viam.com/robotcore/robots/fake"

	// these are the core image things we always want
	_ "go.viam.com/robotcore/rimage" // this is for the core camera types
	_ "go.viam.com/robotcore/vision" // this is for interesting camera types, depth, etc...

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type Robot struct {
	remotes      map[string]api.Robot
	boards       map[string]board.Board
	arms         map[string]api.Arm
	grippers     map[string]api.Gripper
	cameras      map[string]gostream.ImageSource
	lidarDevices map[string]lidar.Device
	bases        map[string]api.Base
	providers    map[string]api.Provider

	config api.Config
	logger golog.Logger
}

func (r *Robot) RemoteByName(name string) api.Robot {
	return r.remotes[name]
}

func (r *Robot) BoardByName(name string) board.Board {
	return r.boards[name]
}

func (r *Robot) ArmByName(name string) api.Arm {
	return r.arms[name]
}

func (r *Robot) BaseByName(name string) api.Base {
	return r.bases[name]
}

func (r *Robot) GripperByName(name string) api.Gripper {
	return r.grippers[name]
}

func (r *Robot) CameraByName(name string) gostream.ImageSource {
	return r.cameras[name]
}

func (r *Robot) LidarDeviceByName(name string) lidar.Device {
	return r.lidarDevices[name]
}

func (r *Robot) ProviderByModel(model string) api.Provider {
	return r.providers[model]
}

func (r *Robot) AddRemote(otherR api.Robot, name string) string {
	if name == "" {
		name = fmt.Sprintf("remote%d", len(r.remotes))
	}
	r.remotes[name] = otherR
	return name
}
func (r *Robot) AddBoard(b board.Board, c board.Config) {
	if c.Name == "" {
		c.Name = fmt.Sprintf("board%d", len(r.boards))
	}
	r.boards[c.Name] = b
}

func (r *Robot) AddArm(a api.Arm, c api.Component) {
	c = fixName(c, api.ComponentTypeArm, len(r.arms))
	r.arms[c.Name] = a
}

func (r *Robot) AddGripper(g api.Gripper, c api.Component) {
	c = fixName(c, api.ComponentTypeGripper, len(r.grippers))
	r.grippers[c.Name] = g
}
func (r *Robot) AddCamera(camera gostream.ImageSource, c api.Component) {
	c = fixName(c, api.ComponentTypeCamera, len(r.cameras))
	r.cameras[c.Name] = camera
}
func (r *Robot) AddLidar(device lidar.Device, c api.Component) {
	c = fixName(c, api.ComponentTypeLidar, len(r.lidarDevices))
	r.lidarDevices[c.Name] = device
}
func (r *Robot) AddBase(b api.Base, c api.Component) {
	c = fixName(c, api.ComponentTypeBase, len(r.bases))
	r.bases[c.Name] = b
}
func (r *Robot) AddProvider(p api.Provider, c api.Component) {
	if c.Name == "" {
		c.Name = fmt.Sprintf("provider%d", len(r.providers))
	}
	r.providers[c.Name] = p
}

func fixName(c api.Component, whichType api.ComponentType, pos int) api.Component {
	if c.Name == "" {
		c.Name = fmt.Sprintf("%s%d", whichType, pos)
	}
	if c.Type == "" {
		c.Type = whichType
	} else if c.Type != whichType {
		panic(fmt.Sprintf("different types (%s) != (%s)", whichType, c.Type))
	}
	return c
}

func (r *Robot) RemoteNames() []string {
	names := []string{}
	for k := range r.remotes {
		names = append(names, k)
	}
	return names
}

func (r *Robot) ArmNames() []string {
	names := []string{}
	for k := range r.arms {
		names = append(names, k)
	}
	return names
}
func (r *Robot) GripperNames() []string {
	names := []string{}
	for k := range r.grippers {
		names = append(names, k)
	}
	return names
}
func (r *Robot) CameraNames() []string {
	names := []string{}
	for k := range r.cameras {
		names = append(names, k)
	}
	return names
}
func (r *Robot) LidarDeviceNames() []string {
	names := []string{}
	for k := range r.lidarDevices {
		names = append(names, k)
	}
	return names
}
func (r *Robot) BaseNames() []string {
	names := []string{}
	for k := range r.bases {
		names = append(names, k)
	}
	return names
}
func (r *Robot) BoardNames() []string {
	names := []string{}
	for k := range r.boards {
		names = append(names, k)
	}
	return names
}

func (r *Robot) Close(ctx context.Context) error {
	for _, x := range r.arms {
		x.Close(ctx)
	}

	for _, x := range r.grippers {
		x.Close(ctx)
	}

	for _, x := range r.cameras {
		x.Close()
	}

	for _, x := range r.lidarDevices {
		if err := x.Close(ctx); err != nil {
			r.logger.Error("error closing lidar device", "error", err)
		}
	}

	for _, x := range r.bases {
		if err := x.Close(ctx); err != nil {
			r.logger.Error("error closing base device", "error", err)
		}
	}

	for _, x := range r.boards {
		if err := x.Close(ctx); err != nil {
			r.logger.Error("error closing boar", "error", err)
		}

	}

	return nil
}

func (r *Robot) GetConfig(ctx context.Context) (api.Config, error) {
	return r.config, nil
}

func (r *Robot) Status(ctx context.Context) (*pb.Status, error) {
	return api.CreateStatus(ctx, r)
}

func (r *Robot) Logger() golog.Logger {
	return r.logger
}

func NewBlankRobot(logger golog.Logger) *Robot {
	return &Robot{
		remotes:      map[string]api.Robot{},
		boards:       map[string]board.Board{},
		arms:         map[string]api.Arm{},
		grippers:     map[string]api.Gripper{},
		cameras:      map[string]gostream.ImageSource{},
		lidarDevices: map[string]lidar.Device{},
		bases:        map[string]api.Base{},
		providers:    map[string]api.Provider{},
		logger:       logger,
	}
}

func NewRobot(ctx context.Context, cfg api.Config, logger golog.Logger) (*Robot, error) {
	r := NewBlankRobot(logger)
	r.config = cfg

	for _, remote := range cfg.Remotes {
		robotClient, err := client.NewRobotClient(ctx, remote.Address, logger)
		if err != nil {
			return nil, err
		}
		name := r.AddRemote(robotClient, remote.Name)
		if err := r.mergeRemote(ctx, robotClient, name, remote.Prefix); err != nil {
			return nil, err
		}
	}

	for _, c := range cfg.Boards {
		b, err := board.NewBoard(ctx, c, logger)
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
			g, err := r.newGripper(c)
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
			lidarDevice, err := r.newLidar(ctx, c, logger)
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

func (r *Robot) mergeRemote(ctx context.Context, otherR api.Robot, robotName string, prefix bool) error {
	status, err := otherR.Status(ctx)
	if err != nil {
		return err
	}
	var prefixName func(name string) string
	if prefix {
		prefixName = func(name string) string { return fmt.Sprintf("%s.%s", robotName, name) }
	} else {
		prefixName = func(name string) string { return name }
	}
	for name := range status.Arms {
		r.AddArm(otherR.ArmByName(name), api.Component{Name: prefixName(name)})
	}
	for name := range status.Bases {
		r.AddBase(otherR.BaseByName(name), api.Component{Name: prefixName(name)})
	}
	for name := range status.Boards {
		r.AddBoard(otherR.BoardByName(name), board.Config{Name: prefixName(name)})
	}
	for name := range status.Cameras {
		r.AddCamera(otherR.CameraByName(name), api.Component{Name: prefixName(name)})
	}
	for name := range status.Grippers {
		r.AddGripper(otherR.GripperByName(name), api.Component{Name: prefixName(name)})
	}
	for name := range status.LidarDevices {
		r.AddLidar(otherR.LidarDeviceByName(name), api.Component{Name: prefixName(name)})
	}
	return nil
}

func (r *Robot) newProvider(config api.Component) (api.Provider, error) {
	pf := api.ProviderLookup(config.Model)
	if pf == nil {
		return nil, fmt.Errorf("unknown provider model: %s", config.Model)
	}
	return pf(r, config, r.logger)
}

func (r *Robot) newBase(config api.Component) (api.Base, error) {
	f := api.BaseLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown base model: %s", config.Model)
	}
	return f(r, config, r.logger)
}

func (r *Robot) newArm(config api.Component) (api.Arm, error) {
	f := api.ArmLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}

	return f(r, config, r.logger)
}

func (r *Robot) newGripper(config api.Component) (api.Gripper, error) {
	f := api.GripperLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
	return f(r, config, r.logger)
}

func (r *Robot) newCamera(config api.Component) (gostream.ImageSource, error) {
	cc := api.CameraLookup(config.Model)
	if cc == nil {
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
	return cc(r, config, r.logger)
}

func (r *Robot) newLidar(ctx context.Context, config api.Component, logger golog.Logger) (lidar.Device, error) {
	var path string
	if config.Host != "" {
		path = fmt.Sprintf("%s:%d", config.Host, config.Port)
	}
	return lidar.CreateDevice(ctx, lidar.DeviceDescription{
		Type: lidar.DeviceType(config.Model),
		Path: path,
	}, logger)
}

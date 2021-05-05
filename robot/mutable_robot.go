// Package robot defines implementations of api.Robot and api.MutableRobot.
//
// It also provides a remote robot implementation that is aware that the api.Robot
// it is working with is not on the same physical system.
package robot

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/sensor"

	// registration
	_ "go.viam.com/robotcore/lidar/client"
	_ "go.viam.com/robotcore/robots/fake"
	_ "go.viam.com/robotcore/sensor/compass/client"
	_ "go.viam.com/robotcore/sensor/compass/gy511"
	_ "go.viam.com/robotcore/sensor/compass/lidar"

	// these are the core image things we always want
	_ "go.viam.com/robotcore/rimage" // this is for the core camera types
	_ "go.viam.com/robotcore/vision" // this is for interesting camera types, depth, etc...

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type mutableRobot struct {
	mu     sync.Mutex
	parts  *robotParts
	config *api.Config
	logger golog.Logger
}

func (r *mutableRobot) RemoteByName(name string) api.Robot {
	return r.parts.RemoteByName(name)
}

func (r *mutableRobot) BoardByName(name string) board.Board {
	return r.parts.BoardByName(name)
}

func (r *mutableRobot) ArmByName(name string) api.Arm {
	return r.parts.ArmByName(name)
}

func (r *mutableRobot) BaseByName(name string) api.Base {
	return r.parts.BaseByName(name)
}

func (r *mutableRobot) GripperByName(name string) api.Gripper {
	return r.parts.GripperByName(name)
}

func (r *mutableRobot) CameraByName(name string) gostream.ImageSource {
	return r.parts.CameraByName(name)
}

func (r *mutableRobot) LidarDeviceByName(name string) lidar.Device {
	return r.parts.LidarDeviceByName(name)
}

func (r *mutableRobot) SensorByName(name string) sensor.Device {
	return r.parts.SensorByName(name)
}

func (r *mutableRobot) ProviderByName(name string) api.Provider {
	return r.parts.ProviderByName(name)
}

func (r *mutableRobot) AddRemote(remote api.Robot, c api.RemoteConfig) {
	r.parts.AddRemote(remote, c)
}

func (r *mutableRobot) AddBoard(b board.Board, c board.Config) {
	r.parts.AddBoard(b, c)
}

func (r *mutableRobot) AddArm(a api.Arm, c api.ComponentConfig) {
	r.parts.AddArm(a, c)
}

func (r *mutableRobot) AddGripper(g api.Gripper, c api.ComponentConfig) {
	r.parts.AddGripper(g, c)
}

func (r *mutableRobot) AddCamera(camera gostream.ImageSource, c api.ComponentConfig) {
	r.parts.AddCamera(camera, c)
}

func (r *mutableRobot) AddLidar(device lidar.Device, c api.ComponentConfig) {
	r.parts.AddLidar(device, c)
}

func (r *mutableRobot) AddBase(b api.Base, c api.ComponentConfig) {
	r.parts.AddBase(b, c)
}

func (r *mutableRobot) AddSensor(s sensor.Device, c api.ComponentConfig) {
	r.parts.AddSensor(s, c)
}

func (r *mutableRobot) AddProvider(p api.Provider, c api.ComponentConfig) {
	r.parts.AddProvider(p, c)
}

func (r *mutableRobot) RemoteNames() []string {
	return r.parts.RemoteNames()
}

func (r *mutableRobot) ArmNames() []string {
	return r.parts.ArmNames()
}

func (r *mutableRobot) GripperNames() []string {
	return r.parts.GripperNames()
}

func (r *mutableRobot) CameraNames() []string {
	return r.parts.CameraNames()
}

func (r *mutableRobot) LidarDeviceNames() []string {
	return r.parts.LidarDeviceNames()
}

func (r *mutableRobot) BaseNames() []string {
	return r.parts.BaseNames()
}

func (r *mutableRobot) BoardNames() []string {
	return r.parts.BoardNames()
}

func (r *mutableRobot) SensorNames() []string {
	return r.parts.SensorNames()
}

func (r *mutableRobot) ProcessManager() rexec.ProcessManager {
	return r.parts.processManager
}

func (r *mutableRobot) Close() error {
	return r.parts.Close()
}

func (r *mutableRobot) GetConfig(ctx context.Context) (*api.Config, error) {
	return r.config, nil
}

func (r *mutableRobot) Status(ctx context.Context) (*pb.Status, error) {
	return api.CreateStatus(ctx, r)
}

func (r *mutableRobot) Logger() golog.Logger {
	return r.logger
}

func NewBlankRobot(logger golog.Logger) api.MutableRobot {
	return &mutableRobot{
		parts:  newRobotParts(logger),
		logger: logger,
	}
}

func NewRobot(ctx context.Context, config *api.Config, logger golog.Logger) (api.MutableRobot, error) {
	r := NewBlankRobot(logger).(*mutableRobot)

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()
	r.config = config

	if err := r.parts.processConfig(ctx, config, r, logger); err != nil {
		return nil, err
	}

	for _, p := range r.parts.providers {
		err := p.Ready(r)
		if err != nil {
			return nil, err
		}
	}

	successful = true
	return r, nil
}

func (r *mutableRobot) newProvider(ctx context.Context, config api.ComponentConfig) (api.Provider, error) {
	f := api.ProviderLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown provider model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newBase(ctx context.Context, config api.ComponentConfig) (api.Base, error) {
	f := api.BaseLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown base model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newArm(ctx context.Context, config api.ComponentConfig) (api.Arm, error) {
	f := api.ArmLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}

	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newGripper(ctx context.Context, config api.ComponentConfig) (api.Gripper, error) {
	f := api.GripperLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newCamera(ctx context.Context, config api.ComponentConfig) (gostream.ImageSource, error) {
	f := api.CameraLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newLidarDevice(ctx context.Context, config api.ComponentConfig) (lidar.Device, error) {
	f := api.LidarDeviceLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown lidar model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newSensor(ctx context.Context, config api.ComponentConfig, sensorType sensor.DeviceType) (sensor.Device, error) {
	f := api.SensorLookup(sensorType, config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown sensor model (type=%s): %s", sensorType, config.Model)
	}
	return f(ctx, r, config, r.logger)
}

// Refresh does nothing for now
func (r *mutableRobot) Refresh(ctx context.Context) error {
	return nil
}

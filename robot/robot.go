package robot

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
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

type Robot struct {
	mu     sync.Mutex
	parts  *robotParts
	config *api.Config
	logger golog.Logger
}

func (r *Robot) RemoteByName(name string) api.RemoteRobot {
	return r.parts.RemoteByName(name)
}

func (r *Robot) BoardByName(name string) board.Board {
	return r.parts.BoardByName(name)
}

func (r *Robot) ArmByName(name string) api.Arm {
	return r.parts.ArmByName(name)
}

func (r *Robot) BaseByName(name string) api.Base {
	return r.parts.BaseByName(name)
}

func (r *Robot) GripperByName(name string) api.Gripper {
	return r.parts.GripperByName(name)
}

func (r *Robot) CameraByName(name string) gostream.ImageSource {
	return r.parts.CameraByName(name)
}

func (r *Robot) LidarDeviceByName(name string) lidar.Device {
	return r.parts.LidarDeviceByName(name)
}

func (r *Robot) SensorByName(name string) sensor.Device {
	return r.parts.SensorByName(name)
}

func (r *Robot) ProviderByName(name string) api.Provider {
	return r.parts.ProviderByName(name)
}

func (r *Robot) AddProvider(p api.Provider, c api.ComponentConfig) {
	r.parts.AddProvider(p, c)
}

func (r *Robot) AddBase(b api.Base, c api.ComponentConfig) {
	r.parts.AddBase(b, c)
}

func (r *Robot) AddCamera(camera gostream.ImageSource, c api.ComponentConfig) {
	r.parts.AddCamera(camera, c)
}

func (r *Robot) RemoteNames() []string {
	return r.parts.RemoteNames()
}

func (r *Robot) ArmNames() []string {
	return r.parts.ArmNames()
}

func (r *Robot) GripperNames() []string {
	return r.parts.GripperNames()
}

func (r *Robot) CameraNames() []string {
	return r.parts.CameraNames()
}

func (r *Robot) LidarDeviceNames() []string {
	return r.parts.LidarDeviceNames()
}

func (r *Robot) BaseNames() []string {
	return r.parts.BaseNames()
}

func (r *Robot) BoardNames() []string {
	return r.parts.BoardNames()
}

func (r *Robot) SensorNames() []string {
	return r.parts.SensorNames()
}

func (r *Robot) Close() error {
	return r.parts.Close()
}

func (r *Robot) GetConfig(ctx context.Context) (*api.Config, error) {
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
		parts:  newRobotParts(logger),
		logger: logger,
	}
}

func NewRobot(ctx context.Context, config *api.Config, logger golog.Logger) (*Robot, error) {
	r := NewBlankRobot(logger)

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

func (r *Robot) newProvider(ctx context.Context, config api.ComponentConfig) (api.Provider, error) {
	f := api.ProviderLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown provider model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *Robot) newBase(ctx context.Context, config api.ComponentConfig) (api.Base, error) {
	f := api.BaseLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown base model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *Robot) newArm(ctx context.Context, config api.ComponentConfig) (api.Arm, error) {
	f := api.ArmLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}

	return f(ctx, r, config, r.logger)
}

func (r *Robot) newGripper(ctx context.Context, config api.ComponentConfig) (api.Gripper, error) {
	f := api.GripperLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *Robot) newCamera(ctx context.Context, config api.ComponentConfig) (gostream.ImageSource, error) {
	f := api.CameraLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *Robot) newLidarDevice(ctx context.Context, config api.ComponentConfig) (lidar.Device, error) {
	f := api.LidarDeviceLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown lidar model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *Robot) newSensor(ctx context.Context, config api.ComponentConfig, sensorType sensor.DeviceType) (sensor.Device, error) {
	f := api.SensorLookup(sensorType, config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown sensor model (type=%s): %s", sensorType, config.Model)
	}
	return f(ctx, r, config, r.logger)
}

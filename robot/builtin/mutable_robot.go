// Package builtinrobot defines implementations of robot.Robot and robot.MutableRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package builtinrobot

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/robotcore/arm"
	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/registry"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/status"

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

// mutableRobot satisfies robot.MutableRobot and defers most
// logic to its parts.
type mutableRobot struct {
	mu     sync.Mutex
	parts  *robotParts
	config *config.Config
	logger golog.Logger
}

func (r *mutableRobot) RemoteByName(name string) robot.Robot {
	return r.parts.RemoteByName(name)
}

func (r *mutableRobot) BoardByName(name string) board.Board {
	return r.parts.BoardByName(name)
}

func (r *mutableRobot) ArmByName(name string) arm.Arm {
	return r.parts.ArmByName(name)
}

func (r *mutableRobot) BaseByName(name string) base.Base {
	return r.parts.BaseByName(name)
}

func (r *mutableRobot) GripperByName(name string) gripper.Gripper {
	return r.parts.GripperByName(name)
}

func (r *mutableRobot) CameraByName(name string) gostream.ImageSource {
	return r.parts.CameraByName(name)
}

func (r *mutableRobot) LidarByName(name string) lidar.Lidar {
	return r.parts.LidarByName(name)
}

func (r *mutableRobot) SensorByName(name string) sensor.Sensor {
	return r.parts.SensorByName(name)
}

func (r *mutableRobot) ProviderByName(name string) robot.Provider {
	return r.parts.ProviderByName(name)
}

func (r *mutableRobot) AddRemote(remote robot.Robot, c config.Remote) {
	r.parts.AddRemote(remote, c)
}

func (r *mutableRobot) AddBoard(b board.Board, c board.Config) {
	r.parts.AddBoard(b, c)
}

func (r *mutableRobot) AddArm(a arm.Arm, c config.Component) {
	r.parts.AddArm(a, c)
}

func (r *mutableRobot) AddGripper(g gripper.Gripper, c config.Component) {
	r.parts.AddGripper(g, c)
}

func (r *mutableRobot) AddCamera(camera gostream.ImageSource, c config.Component) {
	r.parts.AddCamera(camera, c)
}

func (r *mutableRobot) AddLidar(device lidar.Lidar, c config.Component) {
	r.parts.AddLidar(device, c)
}

func (r *mutableRobot) AddBase(b base.Base, c config.Component) {
	r.parts.AddBase(b, c)
}

func (r *mutableRobot) AddSensor(s sensor.Sensor, c config.Component) {
	r.parts.AddSensor(s, c)
}

func (r *mutableRobot) AddProvider(p robot.Provider, c config.Component) {
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

func (r *mutableRobot) LidarNames() []string {
	return r.parts.LidarNames()
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

func (r *mutableRobot) GetConfig(ctx context.Context) (*config.Config, error) {
	return r.config, nil
}

func (r *mutableRobot) Status(ctx context.Context) (*pb.Status, error) {
	return status.Create(ctx, r)
}

func (r *mutableRobot) Logger() golog.Logger {
	return r.logger
}

// NewBlankRobot returns a new robot with no parts.
func NewBlankRobot(logger golog.Logger) robot.MutableRobot {
	return &mutableRobot{
		parts:  newRobotParts(logger),
		logger: logger,
	}
}

// NewRobot returns a new robot with parts sourced from the given config.
func NewRobot(ctx context.Context, config *config.Config, logger golog.Logger) (robot.MutableRobot, error) {
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

func (r *mutableRobot) newProvider(ctx context.Context, config config.Component) (robot.Provider, error) {
	f := registry.ProviderLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown provider model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newBase(ctx context.Context, config config.Component) (base.Base, error) {
	f := registry.BaseLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown base model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newArm(ctx context.Context, config config.Component) (arm.Arm, error) {
	f := registry.ArmLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown arm model: %s", config.Model)
	}

	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newGripper(ctx context.Context, config config.Component) (gripper.Gripper, error) {
	f := registry.GripperLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown gripper model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newCamera(ctx context.Context, config config.Component) (gostream.ImageSource, error) {
	f := registry.CameraLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown camera model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newLidar(ctx context.Context, config config.Component) (lidar.Lidar, error) {
	f := registry.LidarLookup(config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown lidar model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newSensor(ctx context.Context, config config.Component, sensorType sensor.Type) (sensor.Sensor, error) {
	f := registry.SensorLookup(sensorType, config.Model)
	if f == nil {
		return nil, fmt.Errorf("unknown sensor model (type=%s): %s", sensorType, config.Model)
	}
	return f(ctx, r, config, r.logger)
}

// Refresh does nothing for now
func (r *mutableRobot) Refresh(ctx context.Context) error {
	return nil
}

// Package robotimpl defines implementations of robot.Robot and robot.MutableRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"sync"

	"go.viam.com/utils/pexec"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/status"

	// registration
	_ "go.viam.com/core/lidar/client"
	_ "go.viam.com/core/robots/fake"
	_ "go.viam.com/core/sensor/compass/client"
	_ "go.viam.com/core/sensor/compass/gy511"
	_ "go.viam.com/core/sensor/compass/lidar"

	// these are the core image things we always want
	_ "go.viam.com/core/rimage" // this is for the core camera types
	_ "go.viam.com/core/vision" // this is for interesting camera types, depth, etc...

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
)

var _ = robot.MutableRobot(&mutableRobot{})

// mutableRobot satisfies robot.MutableRobot and defers most
// logic to its parts.
type mutableRobot struct {
	mu     sync.Mutex
	parts  *robotParts
	config *config.Config
	logger golog.Logger
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *mutableRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.parts.RemoteByName(name)
}

// BoardByName returns a board by name. If it does not exist
// nil is returned.
func (r *mutableRobot) BoardByName(name string) (board.Board, bool) {
	return r.parts.BoardByName(name)
}

// ArmByName returns an arm by name. If it does not exist
// nil is returned.
func (r *mutableRobot) ArmByName(name string) (arm.Arm, bool) {
	return r.parts.ArmByName(name)
}

// BaseByName returns a base by name. If it does not exist
// nil is returned.
func (r *mutableRobot) BaseByName(name string) (base.Base, bool) {
	return r.parts.BaseByName(name)
}

// GripperByName returns a gripper by name. If it does not exist
// nil is returned.
func (r *mutableRobot) GripperByName(name string) (gripper.Gripper, bool) {
	return r.parts.GripperByName(name)
}

// CameraByName returns a camera by name. If it does not exist
// nil is returned.
func (r *mutableRobot) CameraByName(name string) (camera.Camera, bool) {
	return r.parts.CameraByName(name)
}

// LidarByName returns a lidar by name. If it does not exist
// nil is returned.
func (r *mutableRobot) LidarByName(name string) (lidar.Lidar, bool) {
	return r.parts.LidarByName(name)
}

// SensorByName returns a sensor by name. If it does not exist
// nil is returned.
func (r *mutableRobot) SensorByName(name string) (sensor.Sensor, bool) {
	return r.parts.SensorByName(name)
}

// AddCamera adds a camera to the robot.
func (r *mutableRobot) AddCamera(c camera.Camera, cc config.Component) {
	r.parts.AddCamera(c, cc)
}

// AddBase adds a base to the robot.
func (r *mutableRobot) AddBase(b base.Base, c config.Component) {
	r.parts.AddBase(b, c)
}

// RemoteNames returns the name of all known remote robots.
func (r *mutableRobot) RemoteNames() []string {
	return r.parts.RemoteNames()
}

// ArmNames returns the name of all known arms.
func (r *mutableRobot) ArmNames() []string {
	return r.parts.ArmNames()
}

// GripperNames returns the name of all known grippers.
func (r *mutableRobot) GripperNames() []string {
	return r.parts.GripperNames()
}

// CameraNames returns the name of all known cameras.
func (r *mutableRobot) CameraNames() []string {
	return r.parts.CameraNames()
}

// LidarNames returns the name of all known lidars.
func (r *mutableRobot) LidarNames() []string {
	return r.parts.LidarNames()
}

// BaseNames returns the name of all known bases.
func (r *mutableRobot) BaseNames() []string {
	return r.parts.BaseNames()
}

// BoardNames returns the name of all known boards.
func (r *mutableRobot) BoardNames() []string {
	return r.parts.BoardNames()
}

// SensorNames returns the name of all known sensors.
func (r *mutableRobot) SensorNames() []string {
	return r.parts.SensorNames()
}

// FunctionNames returns the name of all known functions.
func (r *mutableRobot) FunctionNames() []string {
	return r.parts.FunctionNames()
}

// ProcessManager returns the process manager for the robot.
func (r *mutableRobot) ProcessManager() pexec.ProcessManager {
	return r.parts.processManager
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *mutableRobot) Close() error {
	return r.parts.Close()
}

// Config returns the config used to construct the robot.
// This is allowed to be partial or empty.
func (r *mutableRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgCpy := *r.config
	cfgCpy.Components = append([]config.Component{}, cfgCpy.Components...)

	for remoteName, r := range r.parts.remotes {
		rc, err := r.Config(ctx)
		if err != nil {
			return nil, err
		}

		for _, c := range rc.Components {
			if c.Parent == "" {
				for _, rc := range cfgCpy.Remotes {
					if rc.Name == remoteName {
						c.Parent = rc.Parent
						break
					}
				}
			}
			cfgCpy.Components = append(cfgCpy.Components, c)
		}

	}
	return &cfgCpy, nil
}

// Status returns the current status of the robot. Usually you
// should use the CreateStatus helper instead of directly calling
// this.
func (r *mutableRobot) Status(ctx context.Context) (*pb.Status, error) {
	return status.Create(ctx, r)
}

func (r *mutableRobot) FrameLookup(ctx context.Context) (referenceframe.FrameLookup, error) {
	return CreateReferenceFrameLookup(ctx, r)
}

// Logger returns the logger the robot is using.
func (r *mutableRobot) Logger() golog.Logger {
	return r.logger
}

// New returns a new robot with parts sourced from the given config.
func New(ctx context.Context, config *config.Config, logger golog.Logger) (robot.MutableRobot, error) {
	r := &mutableRobot{
		parts:  newRobotParts(logger),
		logger: logger,
	}

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

	successful = true
	return r, nil
}

func (r *mutableRobot) newBase(ctx context.Context, config config.Component) (base.Base, error) {
	f := registry.BaseLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown base model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newArm(ctx context.Context, config config.Component) (arm.Arm, error) {
	f := registry.ArmLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown arm model: %s", config.Model)
	}

	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newGripper(ctx context.Context, config config.Component) (gripper.Gripper, error) {
	f := registry.GripperLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown gripper model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newCamera(ctx context.Context, config config.Component) (camera.Camera, error) {
	f := registry.CameraLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown camera model: %s", config.Model)
	}
	is, err := f(ctx, r, config, r.logger)
	if err != nil {
		return nil, err
	}
	return &camera.ImageSource{is}, nil
}

func (r *mutableRobot) newLidar(ctx context.Context, config config.Component) (lidar.Lidar, error) {
	f := registry.LidarLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown lidar model: %s", config.Model)
	}
	return f(ctx, r, config, r.logger)
}

func (r *mutableRobot) newSensor(ctx context.Context, config config.Component, sensorType sensor.Type) (sensor.Sensor, error) {
	f := registry.SensorLookup(sensorType, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown sensor model (type=%s): %s", sensorType, config.Model)
	}
	return f(ctx, r, config, r.logger)
}

// Refresh does nothing for now
func (r *mutableRobot) Refresh(ctx context.Context) error {
	return nil
}

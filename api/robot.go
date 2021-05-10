// Package api defines the interfaces to configure and work with a robot along with all of its parts.
package api

import (
	"context"
	"fmt"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/sensor"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// RemoteByName returns a remote robot by name. If it does not exist
	// nil is returned.
	RemoteByName(name string) Robot

	// ArmByName returns an arm by name. If it does not exist
	// nil is returned.
	ArmByName(name string) Arm

	// BaseByName returns a base by name. If it does not exist
	// nil is returned.
	BaseByName(name string) Base

	// GripperByName returns a gripper by name. If it does not exist
	// nil is returned.
	GripperByName(name string) Gripper

	// CameraByName returns a camerea by name. If it does not exist
	// nil is returned.
	CameraByName(name string) gostream.ImageSource

	// LidarByName returns a lidar by name. If it does not exist
	// nil is returned.
	LidarByName(name string) lidar.Device

	// BoardByName returns a board by name. If it does not exist
	// nil is returned.
	BoardByName(name string) board.Board

	// SensorByName returns a sensor by name. If it does not exist
	// nil is returned.
	SensorByName(name string) sensor.Device

	// ProviderByName returns a provider by name. If it does not exist
	// nil is returned.
	ProviderByName(name string) Provider

	// RemoteNames returns the name of all known remote robots.
	RemoteNames() []string

	// ArmNames returns the name of all known arms.
	ArmNames() []string

	// GripperNames returns the name of all known grippers.
	GripperNames() []string

	// CameraNames returns the name of all known cameras.
	CameraNames() []string

	// LidarNames returns the name of all known lidars.
	LidarNames() []string

	// BaseNames returns the name of all known bases.
	BaseNames() []string

	// BoardNames returns the name of all known boards.
	BoardNames() []string

	// SensorNames returns the name of all known sensors.
	SensorNames() []string

	// ProcessManager returns the process manager for the robot.
	ProcessManager() rexec.ProcessManager

	// GetConfig returns the config used to construct the robot.
	// This is allowed to be partial or empty.
	GetConfig(ctx context.Context) (*Config, error)

	// Status returns the current status of the robot. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context) (*pb.Status, error)

	// Refresh instructs the Robot to manually refresh the details of itself.
	Refresh(ctx context.Context) error

	// Logger returns the logger the robot is using.
	Logger() golog.Logger
}

// A MutableRobot is a Robot that can have its parts modified.
type MutableRobot interface {
	Robot

	// AddRemote adds a remote robot to the robot.
	AddRemote(remote Robot, c RemoteConfig)

	// AddBoard adds a board to the robot.
	AddBoard(b board.Board, c board.Config)

	// AddArm adds an arm to the robot.
	AddArm(a Arm, c ComponentConfig)

	// AddGripper adds a gripper to the robot.
	AddGripper(g Gripper, c ComponentConfig)

	// AddCamera adds a camera to the robot.
	AddCamera(camera gostream.ImageSource, c ComponentConfig)

	// AddLidar adds a lidar to the robot.
	AddLidar(device lidar.Device, c ComponentConfig)

	// AddBase adds a base to the robot.
	AddBase(b Base, c ComponentConfig)

	// AddSensor adds a sensor to the robot.
	AddSensor(s sensor.Device, c ComponentConfig)

	// AddProvider adds a provider to the robot.
	AddProvider(p Provider, c ComponentConfig)

	// Reconfigure instructs the robot to safely reconfigure itself based
	// on the given new config.
	Reconfigure(ctx context.Context, newConfig *Config) error

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close() error
}

// RobotAsMutable returns a mutable version of the given robot if it
// supports it.
func RobotAsMutable(r Robot) (MutableRobot, error) {
	if m, ok := r.(MutableRobot); ok {
		return m, nil
	}
	return nil, fmt.Errorf("expected %T to be a MutableRobot", r)
}

// A Provider is responsible for providing functionality to parts in a
// robot.
type Provider interface {
	// Ready does any provider/platform initialization once robot configuration is
	// finishing.
	Ready(r Robot) error
}

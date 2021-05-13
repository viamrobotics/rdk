// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"
	"fmt"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rexec"
	"go.viam.com/core/sensor"

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
	ArmByName(name string) arm.Arm

	// BaseByName returns a base by name. If it does not exist
	// nil is returned.
	BaseByName(name string) base.Base

	// GripperByName returns a gripper by name. If it does not exist
	// nil is returned.
	GripperByName(name string) gripper.Gripper

	// CameraByName returns a camera by name. If it does not exist
	// nil is returned.
	CameraByName(name string) gostream.ImageSource

	// LidarByName returns a lidar by name. If it does not exist
	// nil is returned.
	LidarByName(name string) lidar.Lidar

	// BoardByName returns a board by name. If it does not exist
	// nil is returned.
	BoardByName(name string) board.Board

	// SensorByName returns a sensor by name. If it does not exist
	// nil is returned.
	SensorByName(name string) sensor.Sensor

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

	// Config returns the config used to construct the robot.
	// This is allowed to be partial or empty.
	Config(ctx context.Context) (*config.Config, error)

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
	AddRemote(remote Robot, c config.Remote)

	// AddBoard adds a board to the robot.
	AddBoard(b board.Board, c board.Config)

	// AddArm adds an arm to the robot.
	AddArm(a arm.Arm, c config.Component)

	// AddGripper adds a gripper to the robot.
	AddGripper(g gripper.Gripper, c config.Component)

	// AddCamera adds a camera to the robot.
	AddCamera(camera gostream.ImageSource, c config.Component)

	// AddLidar adds a lidar to the robot.
	AddLidar(device lidar.Lidar, c config.Component)

	// AddBase adds a base to the robot.
	AddBase(b base.Base, c config.Component)

	// AddSensor adds a sensor to the robot.
	AddSensor(s sensor.Sensor, c config.Component)

	// AddProvider adds a provider to the robot.
	AddProvider(p Provider, c config.Component)

	// Reconfigure instructs the robot to safely reconfigure itself based
	// on the given new config.
	Reconfigure(ctx context.Context, newConfig *config.Config) error

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close() error
}

// AsMutable returns a mutable version of the given robot if it
// supports it.
func AsMutable(r Robot) (MutableRobot, error) {
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

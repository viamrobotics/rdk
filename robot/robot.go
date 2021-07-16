// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"

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
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// RemoteByName returns a remote robot by name.
	RemoteByName(name string) (Robot, bool)

	// ArmByName returns an arm by name.
	ArmByName(name string) (arm.Arm, bool)

	// BaseByName returns a base by name.
	BaseByName(name string) (base.Base, bool)

	// GripperByName returns a gripper by name.
	GripperByName(name string) (gripper.Gripper, bool)

	// CameraByName returns a camera by name.
	CameraByName(name string) (camera.Camera, bool)

	// LidarByName returns a lidar by name.
	LidarByName(name string) (lidar.Lidar, bool)

	// BoardByName returns a board by name.
	BoardByName(name string) (board.Board, bool)

	// SensorByName returns a sensor by name.
	SensorByName(name string) (sensor.Sensor, bool)

	// ProviderByName returns a provider by name.
	ProviderByName(name string) (Provider, bool)

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
	ProcessManager() pexec.ProcessManager

	// Config returns the config used to construct the robot.
	// This is allowed to be partial or empty.
	Config(ctx context.Context) (*config.Config, error)

	// Status returns the current status of the robot. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context) (*pb.Status, error)

	// FrameLookup returns a FrameLookup suitable for doing reference frame lookups
	// and then computing relative offsets of pieces
	FrameLookup(ctx context.Context) (referenceframe.FrameLookup, error)

	// Logger returns the logger the robot is using.
	Logger() golog.Logger
}

// A Refresher can refresh the contents of a robot.
type Refresher interface {
	// Refresh instructs the Robot to manually refresh the contents of itself.
	Refresh(ctx context.Context) error
}

// A MutableRobot is a Robot that can have its parts modified.
type MutableRobot interface {
	Robot

	// AddBase adds a base to the robot.
	AddBase(b base.Base, c config.Component)

	// AddCamera adds a camera to the robot.
	AddCamera(c camera.Camera, cc config.Component)

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
	return nil, errors.Errorf("expected %T to be a MutableRobot", r)
}

// A Provider is responsible for providing functionality to parts in a
// robot.
type Provider interface {
	// Ready does any provider/platform initialization once robot configuration is
	// finishing.
	Ready(r Robot) error
}

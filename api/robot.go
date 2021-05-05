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
	RemoteByName(name string) Robot
	ArmByName(name string) Arm
	BaseByName(name string) Base
	GripperByName(name string) Gripper
	CameraByName(name string) gostream.ImageSource
	LidarDeviceByName(name string) lidar.Device
	BoardByName(name string) board.Board
	SensorByName(name string) sensor.Device
	ProviderByName(name string) Provider

	RemoteNames() []string
	ArmNames() []string
	GripperNames() []string
	CameraNames() []string
	LidarDeviceNames() []string
	BaseNames() []string
	BoardNames() []string
	SensorNames() []string

	ProcessManager() rexec.ProcessManager

	// this is allowed to be partial or empty
	GetConfig(ctx context.Context) (*Config, error)

	// use CreateStatus helper in most cases
	Status(ctx context.Context) (*pb.Status, error)

	// Refresh instructs the Robot to manually refresh the details of itself.
	Refresh(ctx context.Context) error

	Logger() golog.Logger
}

type MutableRobot interface {
	Robot
	AddRemote(remote Robot, c RemoteConfig)
	AddBoard(b board.Board, c board.Config)
	AddArm(a Arm, c ComponentConfig)
	AddGripper(g Gripper, c ComponentConfig)
	AddCamera(camera gostream.ImageSource, c ComponentConfig)
	AddLidar(device lidar.Device, c ComponentConfig)
	AddBase(b Base, c ComponentConfig)
	AddSensor(s sensor.Device, c ComponentConfig)
	AddProvider(p Provider, c ComponentConfig)
	Reconfigure(ctx context.Context, newConfig *Config) error
	Close() error
}

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

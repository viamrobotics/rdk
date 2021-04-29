package api

import (
	"context"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/sensor"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// providers are for singletons for a whole name
	ProviderByName(name string) Provider
	AddProvider(p Provider, config ComponentConfig)

	RemoteByName(name string) RemoteRobot
	ArmByName(name string) Arm
	BaseByName(name string) Base
	GripperByName(name string) Gripper
	CameraByName(name string) gostream.ImageSource
	LidarDeviceByName(name string) lidar.Device
	BoardByName(name string) board.Board
	SensorByName(name string) sensor.Device

	RemoteNames() []string
	ArmNames() []string
	GripperNames() []string
	CameraNames() []string
	LidarDeviceNames() []string
	BaseNames() []string
	BoardNames() []string
	SensorNames() []string

	// this is allowed to be partial or empty
	GetConfig(ctx context.Context) (*Config, error)

	// use CreateStatus helper in most cases
	Status(ctx context.Context) (*pb.Status, error)

	Logger() golog.Logger
}

// A Provider is responsible for providing functionality to parts in a
// robot.
type Provider interface {
	// Ready does any provider/platform initialization once robot configuration is
	// finishing.
	Ready(r Robot) error
}

// A RemoteRobot is a robot controlled over a network.
type RemoteRobot interface {
	Robot

	// Refresh instructs the RemoteRobot to manually refresh
	// the details of itself based on what the remote endpoint
	// can provide.
	Refresh(ctx context.Context) error
}

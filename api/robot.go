package api

import (
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type Robot interface {
	// providers are for singletons for a whole model
	ProviderByModel(model string) Provider
	AddProvider(p Provider, c Component)

	ArmByName(name string) Arm
	BaseByName(name string) Base
	GripperByName(name string) Gripper
	CameraByName(name string) gostream.ImageSource
	LidarDeviceByName(name string) lidar.Device
	BoardByName(name string) board.Board

	ArmNames() []string
	GripperNames() []string
	CameraNames() []string
	LidarDeviceNames() []string
	BaseNames() []string
	BoardNames() []string

	// this is allowed to be partial or empty
	GetConfig() Config

	// use CreateStatus helper in most cases
	Status() (*pb.Status, error)

	Logger() golog.Logger
}

type Provider interface {
	Ready(r Robot) error
}

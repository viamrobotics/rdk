package api

import (
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
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
	Status() (Status, error)

	Logger() golog.Logger
}

type Provider interface {
	Ready(r Robot) error
}

package api

import (
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
)

type Robot interface {
	// providers are for singletons for a whole model
	ProviderByModel(model string) Provider

	ArmByName(name string) Arm
	GripperByName(name string) Gripper
	CameraByName(name string) gostream.ImageSource
	LidarDeviceByName(name string) lidar.Device
	BoardByName(name string) board.Board

	AddProvider(p Provider, c Component)
}

type Provider interface {
	Ready(r Robot) error
}

package api

import (
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/arm"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/lidar"
)

type Robot interface {
	ArmByName(name string) arm.Arm
	GripperByName(name string) gripper.Gripper
	CameraByName(name string) gostream.ImageSource
	LidarDeviceByName(name string) lidar.Device
	BoardByName(name string) board.Board
}

type Provider interface {
	Ready(r Robot) error
}

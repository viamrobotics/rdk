package inject

import (
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type Robot struct {
	api.Robot
	ProviderByModelFunc   func(model string) api.Provider
	AddProviderFunc       func(p api.Provider, c api.Component)
	ArmByNameFunc         func(name string) api.Arm
	BaseByNameFunc        func(name string) api.Base
	GripperByNameFunc     func(name string) api.Gripper
	CameraByNameFunc      func(name string) gostream.ImageSource
	LidarDeviceByNameFunc func(name string) lidar.Device
	BoardByNameFunc       func(name string) board.Board
	ArmNamesFunc          func() []string
	GripperNamesFunc      func() []string
	CameraNamesFunc       func() []string
	LidarDeviceNamesFunc  func() []string
	BaseNamesFunc         func() []string
	BoardNamesFunc        func() []string
	GetConfigFunc         func() api.Config
	StatusFunc            func() (*pb.Status, error)
	LoggerFunc            func() golog.Logger
}

func (r *Robot) ProviderByModel(model string) api.Provider {
	if r.ProviderByModelFunc == nil {
		return r.Robot.ProviderByModel(model)
	}
	return r.ProviderByModelFunc(model)
}

func (r *Robot) AddProvider(p api.Provider, c api.Component) {
	if r.AddProviderFunc == nil {
		r.Robot.AddProvider(p, c)
		return
	}
	r.AddProviderFunc(p, c)
}

func (r *Robot) ArmByName(name string) api.Arm {
	if r.ArmByNameFunc == nil {
		return r.Robot.ArmByName(name)
	}
	return r.ArmByNameFunc(name)
}

func (r *Robot) BaseByName(name string) api.Base {
	if r.BaseByNameFunc == nil {
		return r.Robot.BaseByName(name)
	}
	return r.BaseByNameFunc(name)
}

func (r *Robot) GripperByName(name string) api.Gripper {
	if r.GripperByNameFunc == nil {
		return r.Robot.GripperByName(name)
	}
	return r.GripperByNameFunc(name)
}

func (r *Robot) CameraByName(name string) gostream.ImageSource {
	if r.CameraByNameFunc == nil {
		return r.Robot.CameraByName(name)
	}
	return r.CameraByNameFunc(name)
}

func (r *Robot) LidarDeviceByName(name string) lidar.Device {
	if r.LidarDeviceByNameFunc == nil {
		return r.Robot.LidarDeviceByName(name)
	}
	return r.LidarDeviceByNameFunc(name)
}

func (r *Robot) BoardByName(name string) board.Board {
	if r.BoardByNameFunc == nil {
		return r.Robot.BoardByName(name)
	}
	return r.BoardByNameFunc(name)
}

func (r *Robot) ArmNames() []string {
	if r.ArmNamesFunc == nil {
		return r.Robot.ArmNames()
	}
	return r.ArmNamesFunc()
}

func (r *Robot) GripperNames() []string {
	if r.GripperNamesFunc == nil {
		return r.Robot.GripperNames()
	}
	return r.GripperNamesFunc()
}

func (r *Robot) CameraNames() []string {
	if r.CameraNamesFunc == nil {
		return r.Robot.CameraNames()
	}
	return r.CameraNamesFunc()
}

func (r *Robot) LidarDeviceNames() []string {
	if r.LidarDeviceNamesFunc == nil {
		return r.Robot.LidarDeviceNames()
	}
	return r.LidarDeviceNamesFunc()
}

func (r *Robot) BaseNames() []string {
	if r.BaseNamesFunc == nil {
		return r.Robot.BaseNames()
	}
	return r.BaseNamesFunc()
}

func (r *Robot) BoardNames() []string {
	if r.BoardNamesFunc == nil {
		return r.Robot.BoardNames()
	}
	return r.BoardNamesFunc()
}

func (r *Robot) GetConfig() api.Config {
	if r.GetConfigFunc == nil {
		return r.Robot.GetConfig()
	}
	return r.GetConfigFunc()
}

func (r *Robot) Status() (*pb.Status, error) {
	if r.StatusFunc == nil {
		return r.Robot.Status()
	}
	return r.StatusFunc()
}

func (r *Robot) Logger() golog.Logger {
	if r.LoggerFunc == nil {
		return r.Robot.Logger()
	}
	return r.LoggerFunc()
}

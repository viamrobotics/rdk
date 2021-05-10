// Package inject provides dependency injected structures for mocking interfaces.
package inject

import (
	"context"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type Robot struct {
	api.Robot
	RemoteByNameFunc   func(name string) api.Robot
	ArmByNameFunc      func(name string) api.Arm
	BaseByNameFunc     func(name string) api.Base
	GripperByNameFunc  func(name string) api.Gripper
	CameraByNameFunc   func(name string) gostream.ImageSource
	LidarByNameFunc    func(name string) lidar.Device
	BoardByNameFunc    func(name string) board.Board
	SensorByNameFunc   func(name string) sensor.Device
	ProviderByNameFunc func(name string) api.Provider
	RemoteNamesFunc    func() []string
	ArmNamesFunc       func() []string
	GripperNamesFunc   func() []string
	CameraNamesFunc    func() []string
	LidarNamesFunc     func() []string
	BaseNamesFunc      func() []string
	BoardNamesFunc     func() []string
	SensorNamesFunc    func() []string
	ProcessManagerFunc func() rexec.ProcessManager
	GetConfigFunc      func(ctx context.Context) (*api.Config, error)
	StatusFunc         func(ctx context.Context) (*pb.Status, error)
	LoggerFunc         func() golog.Logger
	CloseFunc          func() error
	RefreshFunc        func(ctx context.Context) error
}

func (r *Robot) RemoteByName(name string) api.Robot {
	if r.RemoteByNameFunc == nil {
		return r.Robot.RemoteByName(name)
	}
	return r.RemoteByNameFunc(name)
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

func (r *Robot) LidarByName(name string) lidar.Device {
	if r.LidarByNameFunc == nil {
		return r.Robot.LidarByName(name)
	}
	return r.LidarByNameFunc(name)
}

func (r *Robot) BoardByName(name string) board.Board {
	if r.BoardByNameFunc == nil {
		return r.Robot.BoardByName(name)
	}
	return r.BoardByNameFunc(name)
}

func (r *Robot) SensorByName(name string) sensor.Device {
	if r.SensorByNameFunc == nil {
		return r.Robot.SensorByName(name)
	}
	return r.SensorByNameFunc(name)
}

func (r *Robot) ProviderByName(name string) api.Provider {
	if r.ProviderByNameFunc == nil {
		return r.Robot.ProviderByName(name)
	}
	return r.ProviderByNameFunc(name)
}

func (r *Robot) RemoteNames() []string {
	if r.RemoteNamesFunc == nil {
		return r.Robot.RemoteNames()
	}
	return r.RemoteNamesFunc()
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

func (r *Robot) LidarNames() []string {
	if r.LidarNamesFunc == nil {
		return r.Robot.LidarNames()
	}
	return r.LidarNamesFunc()
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

func (r *Robot) SensorNames() []string {
	if r.SensorNamesFunc == nil {
		return r.Robot.SensorNames()
	}
	return r.SensorNamesFunc()
}

func (r *Robot) ProcessManager() rexec.ProcessManager {
	if r.ProcessManagerFunc == nil {
		return r.Robot.ProcessManager()
	}
	return r.ProcessManagerFunc()
}

func (r *Robot) GetConfig(ctx context.Context) (*api.Config, error) {
	if r.GetConfigFunc == nil {
		return r.Robot.GetConfig(ctx)
	}
	return r.GetConfigFunc(ctx)
}

func (r *Robot) Status(ctx context.Context) (*pb.Status, error) {
	if r.StatusFunc == nil {
		return r.Robot.Status(ctx)
	}
	return r.StatusFunc(ctx)
}

func (r *Robot) Logger() golog.Logger {
	if r.LoggerFunc == nil {
		return r.Robot.Logger()
	}
	return r.LoggerFunc()
}

func (r *Robot) Close() error {
	if r.CloseFunc == nil {
		return utils.TryClose(r.Robot)
	}
	return r.CloseFunc()
}

func (r *Robot) Refresh(ctx context.Context) error {
	if r.RefreshFunc == nil {
		if remote, ok := r.Robot.(api.Robot); ok {
			return remote.Refresh(ctx)
		}
	}
	return r.RefreshFunc(ctx)
}

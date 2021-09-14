// Package inject provides dependency injected structures for mocking interfaces.
package inject

import (
	"context"

	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
)

// Robot is an injected robot.
type Robot struct {
	robot.Robot
	RemoteByNameFunc   func(name string) (robot.Robot, bool)
	ArmByNameFunc      func(name string) (arm.Arm, bool)
	BaseByNameFunc     func(name string) (base.Base, bool)
	GripperByNameFunc  func(name string) (gripper.Gripper, bool)
	CameraByNameFunc   func(name string) (camera.Camera, bool)
	LidarByNameFunc    func(name string) (lidar.Lidar, bool)
	BoardByNameFunc    func(name string) (board.Board, bool)
	SensorByNameFunc   func(name string) (sensor.Sensor, bool)
	ProviderByNameFunc func(name string) (robot.Provider, bool)
	RemoteNamesFunc    func() []string
	ArmNamesFunc       func() []string
	GripperNamesFunc   func() []string
	CameraNamesFunc    func() []string
	LidarNamesFunc     func() []string
	BaseNamesFunc      func() []string
	BoardNamesFunc     func() []string
	SensorNamesFunc    func() []string
	ProcessManagerFunc func() pexec.ProcessManager
	ConfigFunc         func(ctx context.Context) (*config.Config, error)
	StatusFunc         func(ctx context.Context) (*pb.Status, error)
	LoggerFunc         func() golog.Logger
	CloseFunc          func() error
	RefreshFunc        func(ctx context.Context) error
}

// RemoteByName calls the injected RemoteByName or the real version.
func (r *Robot) RemoteByName(name string) (robot.Robot, bool) {
	if r.RemoteByNameFunc == nil {
		return r.Robot.RemoteByName(name)
	}
	return r.RemoteByNameFunc(name)
}

// ArmByName calls the injected ArmByName or the real version.
func (r *Robot) ArmByName(name string) (arm.Arm, bool) {
	if r.ArmByNameFunc == nil {
		return r.Robot.ArmByName(name)
	}
	return r.ArmByNameFunc(name)
}

// BaseByName calls the injected BaseByName or the real version.
func (r *Robot) BaseByName(name string) (base.Base, bool) {
	if r.BaseByNameFunc == nil {
		return r.Robot.BaseByName(name)
	}
	return r.BaseByNameFunc(name)
}

// GripperByName calls the injected GripperByName or the real version.
func (r *Robot) GripperByName(name string) (gripper.Gripper, bool) {
	if r.GripperByNameFunc == nil {
		return r.Robot.GripperByName(name)
	}
	return r.GripperByNameFunc(name)
}

// CameraByName calls the injected CameraByName or the real version.
func (r *Robot) CameraByName(name string) (camera.Camera, bool) {
	if r.CameraByNameFunc == nil {
		return r.Robot.CameraByName(name)
	}
	return r.CameraByNameFunc(name)
}

// LidarByName calls the injected LidarByName or the real version.
func (r *Robot) LidarByName(name string) (lidar.Lidar, bool) {
	if r.LidarByNameFunc == nil {
		return r.Robot.LidarByName(name)
	}
	return r.LidarByNameFunc(name)
}

// BoardByName calls the injected BoardByName or the real version.
func (r *Robot) BoardByName(name string) (board.Board, bool) {
	if r.BoardByNameFunc == nil {
		return r.Robot.BoardByName(name)
	}
	return r.BoardByNameFunc(name)
}

// SensorByName calls the injected SensorByName or the real version.
func (r *Robot) SensorByName(name string) (sensor.Sensor, bool) {
	if r.SensorByNameFunc == nil {
		return r.Robot.SensorByName(name)
	}
	return r.SensorByNameFunc(name)
}

// ProviderByName calls the injected ProviderByName or the real version.
func (r *Robot) ProviderByName(name string) (robot.Provider, bool) {
	if r.ProviderByNameFunc == nil {
		return r.Robot.ProviderByName(name)
	}
	return r.ProviderByNameFunc(name)
}

// RemoteNames calls the injected RemoteNames or the real version.
func (r *Robot) RemoteNames() []string {
	if r.RemoteNamesFunc == nil {
		return r.Robot.RemoteNames()
	}
	return r.RemoteNamesFunc()
}

// ArmNames calls the injected ArmNames or the real version.
func (r *Robot) ArmNames() []string {
	if r.ArmNamesFunc == nil {
		return r.Robot.ArmNames()
	}
	return r.ArmNamesFunc()
}

// GripperNames calls the injected GripperNames or the real version.
func (r *Robot) GripperNames() []string {
	if r.GripperNamesFunc == nil {
		return r.Robot.GripperNames()
	}
	return r.GripperNamesFunc()
}

// CameraNames calls the injected CameraNames or the real version.
func (r *Robot) CameraNames() []string {
	if r.CameraNamesFunc == nil {
		return r.Robot.CameraNames()
	}
	return r.CameraNamesFunc()
}

// LidarNames calls the injected LidarNames or the real version.
func (r *Robot) LidarNames() []string {
	if r.LidarNamesFunc == nil {
		return r.Robot.LidarNames()
	}
	return r.LidarNamesFunc()
}

// BaseNames calls the injected BaseNames or the real version.
func (r *Robot) BaseNames() []string {
	if r.BaseNamesFunc == nil {
		return r.Robot.BaseNames()
	}
	return r.BaseNamesFunc()
}

// BoardNames calls the injected BoardNames or the real version.
func (r *Robot) BoardNames() []string {
	if r.BoardNamesFunc == nil {
		return r.Robot.BoardNames()
	}
	return r.BoardNamesFunc()
}

// SensorNames calls the injected SensorNames or the real version.
func (r *Robot) SensorNames() []string {
	if r.SensorNamesFunc == nil {
		return r.Robot.SensorNames()
	}
	return r.SensorNamesFunc()
}

// ProcessManager calls the injected ProcessManager or the real version.
func (r *Robot) ProcessManager() pexec.ProcessManager {
	if r.ProcessManagerFunc == nil {
		return r.Robot.ProcessManager()
	}
	return r.ProcessManagerFunc()
}

// Config calls the injected Config or the real version.
func (r *Robot) Config(ctx context.Context) (*config.Config, error) {
	if r.ConfigFunc == nil {
		return r.Robot.Config(ctx)
	}
	return r.ConfigFunc(ctx)
}

// Status calls the injected Status or the real version.
func (r *Robot) Status(ctx context.Context) (*pb.Status, error) {
	if r.StatusFunc == nil {
		return r.Robot.Status(ctx)
	}
	return r.StatusFunc(ctx)
}

// Logger calls the injected Logger or the real version.
func (r *Robot) Logger() golog.Logger {
	if r.LoggerFunc == nil {
		return r.Robot.Logger()
	}
	return r.LoggerFunc()
}

// Close calls the injected Close or the real version.
func (r *Robot) Close() error {
	if r.CloseFunc == nil {
		return utils.TryClose(r.Robot)
	}
	return r.CloseFunc()
}

// Refresh calls the injected Refresh or the real version.
func (r *Robot) Refresh(ctx context.Context) error {
	if r.RefreshFunc == nil {
		if refresher, ok := r.Robot.(robot.Refresher); ok {
			return refresher.Refresh(ctx)
		}
		return nil
	}
	return r.RefreshFunc(ctx)
}

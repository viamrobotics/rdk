// Package inject provides dependency injected structures for mocking interfaces.
package inject

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// Robot is an injected robot.
type Robot struct {
	robot.Robot
	RemoteByNameFunc          func(name string) (robot.Robot, bool)
	BaseByNameFunc            func(name string) (base.Base, bool)
	GripperByNameFunc         func(name string) (gripper.Gripper, bool)
	CameraByNameFunc          func(name string) (camera.Camera, bool)
	BoardByNameFunc           func(name string) (board.Board, bool)
	MotorByNameFunc           func(name string) (motor.Motor, bool)
	InputControllerByNameFunc func(name string) (input.Controller, bool)
	ResourceByNameFunc        func(name resource.Name) (interface{}, bool)
	RemoteNamesFunc           func() []string
	GripperNamesFunc          func() []string
	CameraNamesFunc           func() []string
	BaseNamesFunc             func() []string
	BoardNamesFunc            func() []string
	MotorNamesFunc            func() []string
	InputControllerNamesFunc  func() []string
	FunctionNamesFunc         func() []string
	FrameSystemFunc           func(ctx context.Context, name string, prefix string) (referenceframe.FrameSystem, error)
	ResourceNamesFunc         func() []resource.Name
	ProcessManagerFunc        func() pexec.ProcessManager
	ConfigFunc                func(ctx context.Context) (*config.Config, error)
	StatusFunc                func(ctx context.Context) (*pb.Status, error)
	LoggerFunc                func() golog.Logger
	CloseFunc                 func(ctx context.Context) error
	RefreshFunc               func(ctx context.Context) error
}

// RemoteByName calls the injected RemoteByName or the real version.
func (r *Robot) RemoteByName(name string) (robot.Robot, bool) {
	if r.RemoteByNameFunc == nil {
		return r.Robot.RemoteByName(name)
	}
	return r.RemoteByNameFunc(name)
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

// BoardByName calls the injected BoardByName or the real version.
func (r *Robot) BoardByName(name string) (board.Board, bool) {
	if r.BoardByNameFunc == nil {
		return r.Robot.BoardByName(name)
	}
	return r.BoardByNameFunc(name)
}

// MotorByName calls the injected MotorByName or the real version.
func (r *Robot) MotorByName(name string) (motor.Motor, bool) {
	if r.MotorByNameFunc == nil {
		return r.Robot.MotorByName(name)
	}
	return r.MotorByNameFunc(name)
}

// InputControllerByName calls the injected InputControllerByName or the real version.
func (r *Robot) InputControllerByName(name string) (input.Controller, bool) {
	if r.InputControllerByNameFunc == nil {
		return r.Robot.InputControllerByName(name)
	}
	return r.InputControllerByNameFunc(name)
}

// ResourceByName calls the injected ResourceByName or the real version.
func (r *Robot) ResourceByName(name resource.Name) (interface{}, bool) {
	if r.ResourceByNameFunc == nil {
		return r.Robot.ResourceByName(name)
	}
	return r.ResourceByNameFunc(name)
}

// RemoteNames calls the injected RemoteNames or the real version.
func (r *Robot) RemoteNames() []string {
	if r.RemoteNamesFunc == nil {
		return r.Robot.RemoteNames()
	}
	return r.RemoteNamesFunc()
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

// MotorNames calls the injected MotorNames or the real version.
func (r *Robot) MotorNames() []string {
	if r.MotorNamesFunc == nil {
		return r.Robot.MotorNames()
	}
	return r.MotorNamesFunc()
}

// InputControllerNames calls the injected InputControllerNames or the real version.
func (r *Robot) InputControllerNames() []string {
	if r.InputControllerNamesFunc == nil {
		return r.Robot.InputControllerNames()
	}
	return r.InputControllerNamesFunc()
}

// FunctionNames calls the injected FunctionNames or the real version.
func (r *Robot) FunctionNames() []string {
	if r.FunctionNamesFunc == nil {
		return r.Robot.FunctionNames()
	}
	return r.FunctionNamesFunc()
}

// FrameSystem calls the injected FrameSystemFunc or the real version.
func (r *Robot) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	if r.FrameSystemFunc == nil {
		return r.Robot.FrameSystem(ctx, name, prefix)
	}
	return r.FrameSystemFunc(ctx, name, prefix)
}

// ResourceNames calls the injected ResourceNames or the real version.
func (r *Robot) ResourceNames() []resource.Name {
	if r.ResourceNamesFunc == nil {
		return r.Robot.ResourceNames()
	}
	return r.ResourceNamesFunc()
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
func (r *Robot) Close(ctx context.Context) error {
	if r.CloseFunc == nil {
		return utils.TryClose(ctx, r.Robot)
	}
	return r.CloseFunc(ctx)
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

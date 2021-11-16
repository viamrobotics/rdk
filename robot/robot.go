// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"

	"go.viam.com/utils/pexec"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/resource"
	"go.viam.com/core/sensor"
	"go.viam.com/core/servo"

	"github.com/edaniels/golog"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// RemoteByName returns a remote robot by name.
	RemoteByName(name string) (Robot, bool)

	// ArmByName returns an arm by name.
	ArmByName(name string) (arm.Arm, bool)

	// BaseByName returns a base by name.
	BaseByName(name string) (base.Base, bool)

	// GripperByName returns a gripper by name.
	GripperByName(name string) (gripper.Gripper, bool)

	// CameraByName returns a camera by name.
	CameraByName(name string) (camera.Camera, bool)

	// LidarByName returns a lidar by name.
	LidarByName(name string) (lidar.Lidar, bool)

	// BoardByName returns a board by name.
	BoardByName(name string) (board.Board, bool)

	// SensorByName returns a sensor by name.
	SensorByName(name string) (sensor.Sensor, bool)

	// ServoByName returns a servo by name.
	ServoByName(name string) (servo.Servo, bool)

	// MotorByName returns a motor by name.
	MotorByName(name string) (motor.Motor, bool)

	// InputControllerByName returns a input.Controller by name.
	InputControllerByName(name string) (input.Controller, bool)

	// ServiceByName returns a service by name.
	// TODO(erd): refactor to service resource
	ServiceByName(name string) (interface{}, bool)

	// ResourceByName returns a resource by name
	ResourceByName(name resource.Name) (interface{}, bool)

	// RemoteNames returns the name of all known remote robots.
	RemoteNames() []string

	// ArmNames returns the name of all known arms.
	ArmNames() []string

	// GripperNames returns the name of all known grippers.
	GripperNames() []string

	// CameraNames returns the name of all known cameras.
	CameraNames() []string

	// LidarNames returns the name of all known lidars.
	LidarNames() []string

	// BaseNames returns the name of all known bases.
	BaseNames() []string

	// BoardNames returns the name of all known boards.
	BoardNames() []string

	// SensorNames returns the name of all known sensors.
	SensorNames() []string

	// ServoNames returns the name of all known servos.
	ServoNames() []string

	// MotorNames returns the name of all known motors.
	MotorNames() []string

	// InputControllerNames returns the name of all known input controllers.
	InputControllerNames() []string

	// FunctionNames returns the name of all known functions.
	FunctionNames() []string

	// ServiceNames returns the name of all known services.
	ServiceNames() []string

	// ResourceNames returns a list of all known resource names
	ResourceNames() []resource.Name

	// ProcessManager returns the process manager for the robot.
	ProcessManager() pexec.ProcessManager

	// Config returns the config used to construct the robot.
	// This is allowed to be partial or empty.
	Config(ctx context.Context) (*config.Config, error)

	// Status returns the current status of the robot. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context) (*pb.Status, error)

	// FrameSystem returns a FrameSystem suitable for doing reference frame lookups
	// and then computing relative offsets of pieces.
	// The frame system will be given a name, and its parts given a prefix (both optional).
	FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error)

	// Logger returns the logger the robot is using.
	Logger() golog.Logger

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close() error
}

// A Refresher can refresh the contents of a robot.
type Refresher interface {
	// Refresh instructs the Robot to manually refresh the contents of itself.
	Refresh(ctx context.Context) error
}

// A LocalRobot is a Robot that can have its parts modified.
type LocalRobot interface {
	Robot

	// Reconfigure instructs the robot to safely reconfigure itself based
	// on the given new config.
	Reconfigure(ctx context.Context, newConfig *config.Config) error
}

//AllResourcesByName returns an array of all resources that have this simple name
func AllResourcesByName(r Robot, name string) []interface{} {
	all := []interface{}{}

	for _, n := range r.ResourceNames() {
		if n.Name == name {
			r, ok := r.ResourceByName(n)
			if !ok {
				panic("this should be impossible")
			}
			all = append(all, r)
		}
	}

	return all
}

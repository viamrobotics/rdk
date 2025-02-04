// Package base defines the base that a robot uses to move around.
// For more information, see the [base component docs].
//
// [base component docs]: https://docs.viam.com/components/base/
package base

import (
	"context"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/component/base/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Base]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterBaseServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.BaseService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string "base".
const SubtypeName = "base"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named Base's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Base represents a physical base of a robot.
// For more information, see the [base component docs].
//
// MoveStraight example:
//
//	myBase, err := base.FromRobot(machine, "my_base")
//	// Move the base forward 40 mm at a velocity of 90 mm/s.
//	myBase.MoveStraight(context.Background(), 40, 90, nil)
//
//	// Move the base backward 40 mm at a velocity of -90 mm/s.
//	myBase.MoveStraight(context.Background(), 40, -90, nil)
//
// For more information, see the [MoveStraight method docs].
//
// Spin example:
//
//	myBase, err := base.FromRobot(machine, "my_base")
//
//	// Spin the base 10 degrees at an angular velocity of 15 deg/sec.
//	myBase.Spin(context.Background(), 10, 15, nil)
//
// For more information, see the [Spin method docs].
//
// SetPower example:
//
//	myBase, err := base.FromRobot(machine, "my_base")
//
//	// Make your wheeled base move forward. Set linear power to 75%.
//	logger.Info("move forward")
//	err = myBase.SetPower(context.Background(), r3.Vector{Y: .75}, r3.Vector{}, nil)
//
//	// Make your wheeled base move backward. Set linear power to -100%.
//	logger.Info("move backward")
//	err = myBase.SetPower(context.Background(), r3.Vector{Y: -1}, r3.Vector{}, nil)
//
//	// Make your wheeled base spin left. Set angular power to 100%.
//	logger.Info("spin left")
//	err = myBase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: 1}, nil)
//
//	// Make your wheeled base spin right. Set angular power to -75%.
//	logger.Info("spin right")
//	err = myBase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: -.75}, nil)
//
// For more information, see the [SetPower method docs].
//
// SetVelocity example:
//
//	myBase, err := base.FromRobot(machine, "my_base")
//
//	// Set the linear velocity to 50 mm/sec and the angular velocity to 15 deg/sec.
//	myBase.SetVelocity(context.Background(), r3.Vector{Y: 50}, r3.Vector{Z: 15}, nil)
//
// For more information, see the [SetVelocity method docs].
//
// Properties example:
//
//	myBase, err := base.FromRobot(machine, "my_base")
//
//	// Get the width and turning radius of the base
//	properties, err := myBase.Properties(context.Background(), nil)
//
//	// Get the width
//	myBaseWidth := properties.WidthMeters
//
//	// Get the turning radius
//	myBaseTurningRadius := properties.TurningRadiusMeters
//
//	// Get the wheel circumference
//	myBaseWheelCircumference := properties.WheelCircumferenceMeters
//
// For more information, see the [Properties method docs].
//
// [base component docs]: https://docs.viam.com/components/base/
// [Properties method docs]: https://docs.viam.com/dev/reference/apis/components/base/#getproperties
// [SetVelocity method docs]: https://docs.viam.com/dev/reference/apis/components/base/#setvelocity
// [SetPower method docs]: https://docs.viam.com/dev/reference/apis/components/base/#setpower
// [Spin method docs]: https://docs.viam.com/dev/reference/apis/components/base/#spin
// [MoveStraight method docs]: https://docs.viam.com/dev/reference/apis/components/base/#movestraight
type Base interface {
	resource.Resource
	resource.Actuator
	resource.Shaped

	// MoveStraight moves the robot straight a given distance at a given speed.
	// If a distance or speed of zero is given, the base will stop.
	// This method blocks until completed or cancelled.
	MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error

	// Spin spins the robot by a given angle in degrees at a given speed.
	// If a speed of 0 the base will stop.
	// Given a positive speed and a positive angle, the base turns to the left (for built-in RDK drivers).
	// This method blocks until completed or cancelled.
	Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error

	// Set the power of the base.
	// For linear power, positive Y moves forwards for built-in RDK drivers.
	// For angular power, positive Z turns to the left for built-in RDK drivers.
	SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	// Set the velocity of the base.
	// linear is in mmPerSec (positive Y moves forwards for built-in RDK drivers).
	// angular is in degsPerSec (positive Z turns to the left for built-in RDK drivers).
	SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	// Properties returns the width, turning radius, and wheel circumference of the physical base in meters.
	Properties(ctx context.Context, extra map[string]interface{}) (Properties, error)
}

// FromDependencies is a helper for getting the named base from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Base, error) {
	return resource.FromDependencies[Base](deps, Named(name))
}

// FromRobot is a helper for getting the named base from the given Robot.
func FromRobot(r robot.Robot, name string) (Base, error) {
	return robot.ResourceFromRobot[Base](r, Named(name))
}

// NamesFromRobot is a helper for getting all base names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// Package base defines the base that a robot uses to move around.
package base

import (
	"context"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/base/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Base]{
		Status:                      resource.StatusFunc(CreateStatus),
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
type Base interface {
	resource.Resource
	resource.Actuator
	resource.Shaped

	// MoveStraight moves the robot straight a given distance at a given speed.
	// If a distance or speed of zero is given, the base will stop.
	// This method blocks until completed or cancelled
	MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error

	// Spin spins the robot by a given angle in degrees at a given speed.
	// If a speed of 0 the base will stop.
	// Given a positive speed and a positive angle, the base turns to the left (for built-in RDK drivers)
	// This method blocks until completed or cancelled
	Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error

	// For linear power, positive Y moves forwards for built-in RDK drivers
	// For angular power, positive Z turns to the left for built-in RDK drivers
	SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	// linear is in mmPerSec (positive Y moves forwards for built-in RDK drivers)
	// angular is in degsPerSec (positive Z turns to the left for built-in RDK drivers)
	SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

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

// CreateStatus creates a status from the base.
func CreateStatus(ctx context.Context, b Base) (*commonpb.ActuatorStatus, error) {
	isMoving, err := b.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &commonpb.ActuatorStatus{IsMoving: isMoving}, nil
}

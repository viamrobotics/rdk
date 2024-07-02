package motor

import (
	"context"
	"fmt"
	"math"

	pb "go.viam.com/api/component/motor/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Motor]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterMotorServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.MotorService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: position.String(),
	}, newPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: isPowered.String(),
	}, newIsPoweredCollector)
}

// SubtypeName is a constant that identifies the component resource API string "motor".
const SubtypeName = "motor"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// A Motor represents a physical motor connected to a board.
//
// SetPower example:
//
//	myMotorComponent, err := motor.FromRobot(machine, "my_motor")
//	// Set the motor power to 40% forwards.
//	myMotorComponent.SetPower(context.Background(), 0.4, nil)
//
// GoFor example:
//
//	myMotorComponent, err := motor.FromRobot(machine, "my_motor")
//	// Turn the motor 7.2 revolutions at 60 RPM.
//	myMotorComponent.GoFor(context.Background(), 60, 7.2, nil)
//
// GoTo example:
//
//	// Turn the motor to 8.3 revolutions from home at 75 RPM.
//	myMotorComponent.GoTo(context.Background(), 75, 8.3, nil)
//
// ResetZeroPosition example:
//
//	// Set the current position as the new home position with no offset.
//	myMotorComponent.ResetZeroPosition(context.Background(), 0.0, nil)
//
// Position example:
//
//	// Get the current position of an encoded motor.
//	position, err := myMotorComponent.Position(context.Background(), nil)
//
//	// Log the position
//	logger.Info("Position:")
//	logger.Info(position)
//
// Properties example:
//
//	// Return whether or not the motor supports certain optional features.
//	properties, err := myMotorComponent.Properties(context.Background(), nil)
//
//	// Log the properties.
//	logger.Info("Properties:")
//	logger.Info(properties)
//
// IsPowered example:
//
//	// Check whether the motor is currently running.
//	powered, pct, err := myMotorComponent.IsPowered(context.Background(), nil)
//
//	logger.Info("Is powered?")
//	logger.Info(powered)
//	logger.Info("Power percent:")
//	logger.Info(pct)
type Motor interface {
	resource.Resource
	resource.Actuator

	// SetPower sets the percentage of power the motor should employ between -1 and 1.
	// Negative power corresponds to a backward direction of rotation.
	SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
	// can be assigned negative values to move in a backwards direction. Note: if both are
	// negative the motor will spin in the forward direction.
	// If revolutions != 0, this will block until the number of revolutions has been completed or another operation comes in.
	// Deprecated: If revolutions is 0, this will run the motor at rpm indefinitely.
	GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
	// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
	// towards the specified target/position.
	// This will block until the position has been reached.
	GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error

	// SetRPM instructs the motor to move at the specified RPM indefinitely.
	SetRPM(ctx context.Context, rpm float64, extra map[string]interface{}) error

	// Set an encoded motor's current position (+/- offset) to be the new zero (home) position.
	ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error

	// Position reports the position of an encoded motor based on its encoder. If it's not supported,
	// the returned data is undefined. The unit returned is the number of revolutions which is
	// intended to be fed back into calls of GoFor.
	Position(ctx context.Context, extra map[string]interface{}) (float64, error)

	// Properties returns whether or not the motor supports certain optional properties.
	Properties(ctx context.Context, extra map[string]interface{}) (Properties, error)

	// IsPowered returns whether or not the motor is currently on, and the percent power (between 0
	// and 1, if the motor is off then the percent power will be 0).
	IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error)
}

// Named is a helper for getting the named Motor's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromDependencies is a helper for getting the named motor from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Motor, error) {
	return resource.FromDependencies[Motor](deps, Named(name))
}

// FromRobot is a helper for getting the named motor from the given Robot.
func FromRobot(r robot.Robot, name string) (Motor, error) {
	return robot.ResourceFromRobot[Motor](r, Named(name))
}

// NamesFromRobot is a helper for getting all motor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// CreateStatus creates a status from the motor.
func CreateStatus(ctx context.Context, m Motor) (*pb.Status, error) {
	isPowered, _, err := m.IsPowered(ctx, nil)
	if err != nil {
		return nil, err
	}
	properties, err := m.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	var position float64
	if properties.PositionReporting {
		position, err = m.Position(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	isMoving, err := m.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{
		IsPowered: isPowered,
		Position:  position,
		IsMoving:  isMoving,
	}, nil
}

// CheckSpeed checks if the input rpm is too slow or fast and returns a warning and/or error.
func CheckSpeed(rpm, max float64) (string, error) {
	switch speed := math.Abs(rpm); {
	case speed < 0.1:
		return "motor speed is nearly 0 rev_per_min", NewZeroRPMError()
	case max > 0 && speed > max-0.1:
		return fmt.Sprintf("motor speed is nearly the max rev_per_min (%f)", max), nil
	default:
		return "", nil
	}
}

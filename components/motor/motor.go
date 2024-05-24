package motor

import (
	"context"

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
//	// Set the motor power to 40% forwards
//	myMotor.SetPower(context.Background(), 0.4, nil)
//
// GoFor example:
//
//	// Turn the motor 7.2 revolutions at 60 RPM
//	myMotor.GoFor(context.Background(), 60, 7.2, nil)
//
// GoTo example:
//
//	// Turn the motor to 8.3 revolutions from home at 75 RPM
//	myMotor.GoTo(context.Background(), 75, 8.3, nil)
//
// ResetZeroPostion example:
//
//	// Set the current position as the new home position with no offset
//	myMotor.ResetZeroPosition(context.Background(), 0.0, nil)
//
// Position example:
//
//	// Get the current position of an encoded motor
//	position, err := myMotor.Position(context.Background(), nil)
//
//	// Log the position
//	logger.Info("Position:")
//	logger.Info(position)
//
// Properties example:
//
//	// Return whether or not the motor supports certain optional features
//	properties, err := myMotor.Properties(context.Background(), nil)
//
//	// Log the properties.
//	logger.Info("Properties:")
//	logger.Info(properties)
//
// IsPowered example:
//
//	// Check whether the motor is currently running
//	powered, pct, err := myMotor.IsPowered(context.Background(), nil)
//
//	logger.Info("Is powered?")
//	logger.Info(powered)
//	logger.Info("Power percent:")
//	logger.Info(pct)
type Motor interface {
	resource.Resource
	resource.Actuator

	// SetPower sets the percentage of power the motor should employ between -1 and 1.
	// Negative power corresponds to a backward direction of rotation
	SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error

	// GoFor instructs the motor to go in a specified direction for a set number of revolutions at a given speed in RPM.
	// Both RPM and revolutions can be negative to move the motor backward. If both are negative, the motor will spin forward.
	// If revolutions is 0, the motor runs at the given RPM indefinitely.
	// If revolutions is non-zero, the motor runs until the specified amount of revolutions is completed or interrupted.
	GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero) at a given speed.
	// Regardless of RPM direction, the motor will move towards the target position.
	// This method blocks until the position has been reached.
	GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error

	// ResetZeroPosition sets an encoded motor's current position (+/- offset) to be the new zero (home) position.
	ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error

	// Position returns the position of an encoded motor based on its encoder.
	// If the motor is not supported, the returned data is undefined.
	// The unit returned is the number of revolutions which is intended to be used with GoFor.
	Position(ctx context.Context, extra map[string]interface{}) (float64, error)

	// Properties returns whether or not the motor supports certain optional properties.
	Properties(ctx context.Context, extra map[string]interface{}) (Properties, error)

	// IsPowered returns whether or not the motor is currently on, and the power percentage.
	// The power percentage value is in the range of 0 and 1. If the motor is off then the power percentage will be 0.
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

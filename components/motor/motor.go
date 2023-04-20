package motor

import (
	"context"

	pb "go.viam.com/api/component/motor/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Motor]{
		Status:                      registry.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterMotorServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.MotorService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: position.String(),
	}, newPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: isPowered.String(),
	}, newIsPoweredCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "motor".
const SubtypeName = resource.SubtypeName("motor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Motor represents a physical motor connected to a board.
type Motor interface {
	resource.Resource
	resource.Actuator

	// SetPower sets the percentage of power the motor should employ between -1 and 1.
	// Negative power implies a backward directional rotational
	SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
	// can be assigned negative values to move in a backwards direction. Note: if both are
	// negative the motor will spin in the forward direction.
	// If revolutions is 0, this will run the motor at rpm indefinitely
	// If revolutions != 0, this will block until the number of revolutions has been completed or another operation comes in.
	GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
	// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
	// towards the specified target/position
	// This will block until the position has been reached
	GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error

	// Set the current position (+/- offset) to be the new zero (home) position.
	ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error

	// Position reports the position of the motor based on its encoder. If it's not supported, the returned
	// data is undefined. The unit returned is the number of revolutions which is intended to be fed
	// back into calls of GoFor.
	Position(ctx context.Context, extra map[string]interface{}) (float64, error)

	// Properties returns whether or not the motor supports certain optional features.
	Properties(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error)

	// IsPowered returns whether or not the motor is currently on, and the percent power (between 0
	// and 1, if the motor is off then the percent power will be 0).
	IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error)
}

// A LocalMotor is a motor that supports additional features provided by RDK
// (e.g. GoTillStop).
type LocalMotor interface {
	Motor
	// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
	// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
	// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
	// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
	GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error
}

// Named is a helper for getting the named Motor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
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
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the motor.
func CreateStatus(ctx context.Context, m Motor) (*pb.Status, error) {
	isPowered, _, err := m.IsPowered(ctx, nil)
	if err != nil {
		return nil, err
	}
	features, err := m.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	var position float64
	if features[PositionReporting] {
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

package servo

import (
	"context"

	pb "go.viam.com/api/component/servo/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterSubtype(Subtype, resource.SubtypeRegistration[Servo]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterServoServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.ServoService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: position.String(),
	}, newPositionCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "servo".
const SubtypeName = resource.SubtypeName("servo")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Servo represents a physical servo connected to a board.
type Servo interface {
	resource.Resource
	resource.Actuator

	// Move moves the servo to the given angle (0-180 degrees)
	// This will block until done or a new operation cancels this one
	Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error

	// Position returns the current set angle (degrees) of the servo.
	Position(ctx context.Context, extra map[string]interface{}) (uint32, error)
}

// Named is a helper for getting the named Servo's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named servo from the given Robot.
func FromRobot(r robot.Robot, name string) (Servo, error) {
	return robot.ResourceFromRobot[Servo](r, Named(name))
}

// NamesFromRobot is a helper for getting all servo names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the servo.
func CreateStatus(ctx context.Context, s Servo) (*pb.Status, error) {
	position, err := s.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	isMoving, err := s.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{PositionDeg: position, IsMoving: isMoving}, nil
}

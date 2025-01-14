// Package servo supports “RC” or “hobby” servo motors.
// For more information, see the [servo component docs].
//
// [servo component docs]: https://docs.viam.com/components/servo/
package servo

import (
	"context"

	pb "go.viam.com/api/component/servo/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Servo]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterServoServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.ServoService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: position.String(),
	}, newPositionCollector)
}

// SubtypeName is a constant that identifies the component resource API string "servo".
const SubtypeName = "servo"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// A Servo represents a physical servo connected to a board.
// For more information, see the [servo component docs].
//
// Move example:
//
//	// Move the servo from its origin to the desired angle of 30 degrees.
//	myServoComponent.Move(context.Background(), 30, nil)
//
// For more information, see the [Move method docs].
//
// Position example:
//
//	// Get the current set angle of the servo.
//	pos1, err := myServoComponent.Position(context.Background(), nil)
//
//	// Move the servo from its origin to the desired angle of 20 degrees.
//	myServoComponent.Move(context.Background(), 20, nil)
//
//	// Get the current set angle of the servo.
//	pos2, err := myServoComponent.Position(context.Background(), nil)
//
//	logger.Info("Position 1: ", pos1)
//	logger.Info("Position 2: ", pos2)
//
// For more information, see the [Position method docs].
//
// [servo component docs]: https://docs.viam.com/dev/reference/apis/components/servo/
// [Move method docs]: https://docs.viam.com/dev/reference/apis/components/servo/#move
// [Position method docs]: https://docs.viam.com/dev/reference/apis/components/servo/#getposition
type Servo interface {
	resource.Resource
	resource.Actuator

	// Move moves the servo to the given angle (0-180 degrees).
	// This will block until done or a new operation cancels this one.
	Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error

	// Position returns the current set angle (degrees) of the servo.
	Position(ctx context.Context, extra map[string]interface{}) (uint32, error)
}

// Named is a helper for getting the named Servo's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named servo from the given Robot.
func FromRobot(r robot.Robot, name string) (Servo, error) {
	return robot.ResourceFromRobot[Servo](r, Named(name))
}

// NamesFromRobot is a helper for getting all servo names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

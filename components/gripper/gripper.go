// Package gripper defines a robotic gripper.
package gripper

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Gripper]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterGripperServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.GripperService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string.
const SubtypeName = "gripper"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named grippers's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Gripper represents a physical robotic gripper.
//
// Open example:
//
//	// Open the gripper.
//	err := myGripper.Open(context.Background(), nil)
//
// Grab example:
//
//	// Grab with the gripper.
//	grabbed, err := myGripper.Grab(context.Background(), nil)
type Gripper interface {
	resource.Resource
	resource.Shaped
	resource.Actuator
	referenceframe.ModelFramer

	// Open opens the gripper.
	// This will block until done or a new operation cancels this one
	Open(ctx context.Context, extra map[string]interface{}) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	// This will block until done or a new operation cancels this one
	Grab(ctx context.Context, extra map[string]interface{}) (bool, error)
}

// FromRobot is a helper for getting the named Gripper from the given Robot.
func FromRobot(r robot.Robot, name string) (Gripper, error) {
	return robot.ResourceFromRobot[Gripper](r, Named(name))
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// CreateStatus creates a status from the gripper.
func CreateStatus(ctx context.Context, g Gripper) (*commonpb.ActuatorStatus, error) {
	isMoving, err := g.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &commonpb.ActuatorStatus{IsMoving: isMoving}, nil
}

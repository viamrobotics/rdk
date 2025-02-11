// Package button defines a button on your machine.
package button

import (
	"context"

	pb "go.viam.com/api/component/button/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Button]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterButtonServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.ButtonService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string.
const SubtypeName = "button"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named grippers's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Button represents a physical button.
type Button interface {
	resource.Resource

	// Push pushes the button.
	// This will block until done or a new operation cancels this one.
	Push(ctx context.Context, extra map[string]interface{}) error
}

// FromRobot is a helper for getting the named Button from the given Robot.
func FromRobot(r robot.Robot, name string) (Button, error) {
	return robot.ResourceFromRobot[Button](r, Named(name))
}

// FromDependencies is a helper for getting the named button component from a collection of dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Button, error) {
	return resource.FromDependencies[Button](deps, Named(name))
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

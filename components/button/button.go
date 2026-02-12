// Package button defines a button on your machine.
package button

import (
	"context"

	pb "go.viam.com/api/component/button/v1"

	"go.viam.com/rdk/data"
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
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
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
// For more information, see the [Button component docs].
//
// Push example:
//
//	myButton, err := switch.FromProvider(machine, "my_button")
//
//	err := myButton.Push(context.Background(), nil)
//
// For more information, see the [Push method docs].
//
// [Button component docs]: https://docs.viam.com/dev/reference/apis/components/button/
// [Push method docs]: https://docs.viam.com/dev/reference/apis/components/button/#push
type Button interface {
	resource.Resource

	// Push pushes the button.
	Push(ctx context.Context, extra map[string]interface{}) error
}

// Deprecated: FromRobot is a helper for getting the named Button from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Button, error) {
	return robot.ResourceFromRobot[Button](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named button component from a collection of dependencies.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (Button, error) {
	return resource.FromDependencies[Button](deps, Named(name))
}

// FromProvider is a helper for getting the named Button from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Button, error) {
	return resource.FromProvider[Button](provider, Named(name))
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// Package toggleswitch defines a multi-position switch.
package toggleswitch

import (
	"context"

	pb "go.viam.com/api/component/switch/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Switch]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterSwitchServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.SwitchService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// SubtypeName is a constant that identifies the component resource API string.
const SubtypeName = "switch"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named switch's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Switch represents a physical multi-position switch.
// For more information, see the [Switch component docs].
//
// SetPosition example:
//
//	mySwitch, err := switch.FromProvider(machine, "my_switch")
//
//	err := mySwitch.SetPosition(context.Background(), 0 , nil)
//
// For more information, see the [SetPosition method docs].
//
// GetPosition example:
//
//	mySwitch, err := switch.FromProvider(machine, "my_switch")
//
//	position, err := mySwitch.GetPosition(context.Background(), nil)
//
// For more information, see the [GetPosition method docs].
//
// GetNumberOfPositions example:
//
//	mySwitch, err := switch.FromProvider(machine, "my_switch")
//
//	positions, err := mySwitch.GetNumberOfPositions(context.Background(), nil)
//
// For more information, see the [GetNumberOfPositions method docs].
//
// [Switch component docs]: https://docs.viam.com/dev/reference/apis/components/switch/
// [SetPosition method docs]: https://docs.viam.com/dev/reference/apis/components/switch/#setposition
// [GetPosition method docs]: https://docs.viam.com/dev/reference/apis/components/switch/#getposition
// [GetNumberOfPositions method docs]: https://docs.viam.com/dev/reference/apis/components/switch/#getnumberofpositions
type Switch interface {
	resource.Resource

	// SetPosition sets the switch to the specified position.
	// Position must be within the valid range for the switch type.
	SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error

	// GetPosition returns the current position of the switch.
	GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error)

	// GetNumberOfPositions returns the total number of valid positions for this switch, along with their labels.
	// Labels should either be nil, empty, or the same length has the number of positions.
	GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error)
}

// Deprecated: FromRobot is a helper for getting the named Switch from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Switch, error) {
	return robot.ResourceFromRobot[Switch](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named switch component from a collection of dependencies.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (Switch, error) {
	return resource.FromDependencies[Switch](deps, Named(name))
}

// FromProvider is a helper for getting the named Switch from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Switch, error) {
	return resource.FromProvider[Switch](provider, Named(name))
}

// NamesFromRobot is a helper for getting all switch names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

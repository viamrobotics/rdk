// Package generic defines an abstract generic device and DoCommand() method.
// For more information, see the [generic component docs].
//
// [generic component docs]: https://docs.viam.com/components/generic/
package generic

import (
	pb "go.viam.com/api/component/generic/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[resource.Resource]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterGenericServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.GenericService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string "Generic".
const SubtypeName = "generic"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named Generic's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named Generic from the given Robot.
func FromRobot(r robot.Robot, name string) (resource.Resource, error) {
	return robot.ResourceFromRobot[resource.Resource](r, Named(name))
}

// NamesFromRobot is a helper for getting all generic names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

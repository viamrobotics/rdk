// Package generic defines an abstract generic service and DoCommand() method.
// For more information, see the [generic service docs].
//
// [generic service docs]: https://docs.viam.com/dev/reference/apis/services/generic/
package generic

import (
	pb "go.viam.com/api/service/generic/v1"

	"go.viam.com/rdk/data"
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
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// SubtypeName is a constant that identifies the service resource API string "Generic".
const SubtypeName = "generic"

// API is a variable that identifies the service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named Generic's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Deprecated: FromRobot is a helper for getting the named Generic from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (resource.Resource, error) {
	return robot.ResourceFromRobot[resource.Resource](r, Named(name))
}

// FromProvider is a helper for getting the named Generic
// from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (resource.Resource, error) {
	return resource.FromProvider[Service](provider, Named(name))
}

// NamesFromRobot is a helper for getting all generic names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

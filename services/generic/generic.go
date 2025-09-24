// Package generic defines an abstract generic service and DoCommand() method.
// For more information, see the [generic service docs].
//
// [generic service docs]: https://docs.viam.com/dev/reference/apis/services/generic/
package generic

import (
	"fmt"

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

// GetResource is a helper for getting the named Generic from either a collection of dependencies
// or the given robot.
func GetResource(src any, name string) (resource.Resource, error) {
	switch v := src.(type) {
	case resource.Dependencies:
		return resource.FromDependencies[resource.Resource](v, Named(name))
	case robot.Robot:
		return robot.ResourceFromRobot[resource.Resource](v, Named(name))
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}


// NamesFromRobot is a helper for getting all generic names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

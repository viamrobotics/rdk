// Package toggleswitch defines a multi-position switch.
package toggleswitch

import (
	"context"
	"fmt"

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

// GetResource is a helper for getting the named Switch from either a collection of dependencies
// or the given robot.
func GetResource(src any, name string) (Switch, error) {
	switch v := src.(type) {
	case resource.Dependencies:
		return resource.FromDependencies[Switch](v, Named(name))
	case robot.Robot:
		return robot.ResourceFromRobot[Switch](v, Named(name))
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}

// NamesFromRobot is a helper for getting all switch names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

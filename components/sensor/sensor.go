// Package sensor defines an abstract sensing device that can provide measurement readings.
// For more information, see the [sensor component docs].
//
// [sensor component docs]: https://docs.viam.com/dev/reference/apis/components/sensor/
package sensor

import (
	"fmt"

	pb "go.viam.com/api/component/sensor/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Sensor]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterSensorServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.SensorService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: readings.String(),
	}, newReadingsCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// SubtypeName is a constant that identifies the component resource API string "Sensor".
const SubtypeName = "sensor"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named Sensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Sensor represents a general purpose sensors that can give arbitrary readings
// of some thing that it is sensing.
type Sensor interface {
	resource.Resource
	resource.Sensor
}

// GetResource is a helper for getting the named Sensor from either a collection of dependencies
// or the given robot.
func GetResource(src any, name string) (Sensor, error) {
	switch v := src.(type) {
	case resource.Dependencies:
		return resource.FromDependencies[Sensor](v, Named(name))
	case robot.Robot:
		return robot.ResourceFromRobot[Sensor](v, Named(name))
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}

// NamesFromRobot is a helper for getting all sensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

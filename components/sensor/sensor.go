// Package sensor defines an abstract sensing device that can provide measurement readings.
package sensor

import (
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
	}, newSensorCollector)
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
	// Readings return data specific to the type of sensor and can be of any type.
	// Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
}

// FromDependencies is a helper for getting the named sensor from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Sensor, error) {
	return resource.FromDependencies[Sensor](deps, Named(name))
}

// FromRobot is a helper for getting the named Sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (Sensor, error) {
	return robot.ResourceFromRobot[Sensor](r, Named(name))
}

// NamesFromRobot is a helper for getting all sensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// Package powersensor defines the interfaces of a powersensor.
// For more information, see the [power sensor component docs].
//
// [power sensor component docs]: https://docs.viam.com/components/power-sensor/
package powersensor

import (
	"context"

	pb "go.viam.com/api/component/powersensor/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[PowerSensor]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterPowerSensorServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.PowerSensorService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: voltage.String(),
	}, newVoltageCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: current.String(),
	}, newCurrentCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: power.String(),
	}, newPowerCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: readings.String(),
	}, newReadingsCollector)
}

// SubtypeName is a constant that identifies the component resource API string "power_sensor".
const SubtypeName = "power_sensor"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named PowerSensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A PowerSensor reports information about voltage, current and power.
// For more information, see the [power sensor component docs].
//
// Voltage example:
//
//	// Get the voltage from device in volts.
//	voltage, isAC, err := myPowerSensor.Voltage(context.Background(), nil)
//
// For more information, see the [Voltage method docs].
//
// Current example:
//
//	// Get the current reading from device in amps.
//	current, isAC, err := myPowerSensor.Current(context.Background(), nil)
//
// For more information, see the [Current method docs].
//
// Power example:
//
//	// Get the power measurement from device in watts.
//	power, err := myPowerSensor.Power(context.Background(), nil)
//
// For more information, see the [Power method docs].
//
// [power sensor component docs]: https://docs.viam.com/dev/reference/apis/components/power-sensor/
// [Voltage method docs]: https://docs.viam.com/dev/reference/apis/components/power-sensor/#getvoltage
// [Current method docs]: https://docs.viam.com/dev/reference/apis/components/power-sensor/#getcurrent
// [Power method docs]: https://docs.viam.com/dev/reference/apis/components/power-sensor/#getpower
type PowerSensor interface {
	resource.Sensor
	resource.Resource
	// Voltage returns the voltage reading in volts and a bool returning true if the voltage is AC.
	Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error)

	// Current returns the current reading in amperes and a bool returning true if the current is AC.
	Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error)

	// Power returns the power reading in watts.
	Power(ctx context.Context, extra map[string]interface{}) (float64, error)
}

// FromDependencies is a helper for getting the named PowerSensor from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (PowerSensor, error) {
	return resource.FromDependencies[PowerSensor](deps, Named(name))
}

// FromRobot is a helper for getting the named PowerSensor from the given Robot.
func FromRobot(r robot.Robot, name string) (PowerSensor, error) {
	return robot.ResourceFromRobot[PowerSensor](r, Named(name))
}

// NamesFromRobot is a helper for getting all PowerSensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// Package powersensor defines the interfaces of a powersensor
package powersensor

import (
	"context"
	"strings"

	pb "go.viam.com/api/component/powersensor/v1"

	"go.viam.com/rdk/components/sensor"
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
type PowerSensor interface {
	sensor.Sensor
	Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error)
	Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error)
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

// Readings is a helper for getting all readings from a PowerSensor.
func Readings(ctx context.Context, g PowerSensor, extra map[string]interface{}) (map[string]interface{}, error) {
	readings := map[string]interface{}{}

	vol, isAC, err := g.Voltage(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedVoltage.Error()) {
			return nil, err
		}
	} else {
		readings["voltage"] = vol
		readings["is_ac"] = isAC
	}

	cur, isAC, err := g.Current(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedCurrent.Error()) {
			return nil, err
		}
	} else {
		readings["current"] = cur
		readings["is_ac"] = isAC
	}

	pow, err := g.Power(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedPower.Error()) {
			return nil, err
		}
	} else {
		readings["power"] = pow
	}

	return readings, nil
}

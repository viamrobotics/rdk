// Package sensor defines an abstract sensing device that can provide measurement readings.
package sensor

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/sensor/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Sensor]{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeColl resource.SubtypeCollection[Sensor]) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SensorService_ServiceDesc,
				NewServer(subtypeColl),
				pb.RegisterSensorServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SensorService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (Sensor, error) {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: readings.String(),
	}, newSensorCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "Sensor".
const SubtypeName = resource.SubtypeName("sensor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Sensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Sensor represents a general purpose sensors that can give arbitrary readings
// of some thing that it is sensing.
type Sensor interface {
	resource.Resource
	// Readings return data specific to the type of sensor and can be of any type.
	Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
}

// FromRobot is a helper for getting the named Sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (Sensor, error) {
	return robot.ResourceFromRobot[Sensor](r, Named(name))
}

// NamesFromRobot is a helper for getting all sensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

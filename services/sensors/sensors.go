// Package sensors implements a sensors service.
package sensors

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/sensors/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SensorsService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterSensorsServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SensorsService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (resource.Resource, error) {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		MaxInstance: resource.DefaultMaxInstance,
	})
}

// A Readings ties both the sensor name and its reading together.
type Readings struct {
	Name     resource.Name
	Readings map[string]interface{}
}

// A Service centralizes all sensors into one place.
type Service interface {
	resource.Resource
	Sensors(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error)
	Readings(ctx context.Context, sensorNames []resource.Name, extra map[string]interface{}) ([]Readings, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("sensors")

// Subtype is a constant that identifies the sensor service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named sensor's typed resource name.
// RSDK-347 Implements senors's Named.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named sensor service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FindFirstName returns name of first sensors service found.
func FindFirstName(r robot.Robot) string {
	for _, val := range robot.NamesBySubtype(r, Subtype) {
		return val
	}
	return ""
}

// FirstFromRobot returns the first sensor service in this robot.
func FirstFromRobot(r robot.Robot) (Service, error) {
	name := FindFirstName(r)
	return FromRobot(r, name)
}

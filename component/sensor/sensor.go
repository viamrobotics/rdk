// Package sensor defines an abstract sensing device that can provide measurement readings.
package sensor

import (
	"context"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/sensor/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SensorService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterSensorServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SensorService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "Sensor".
const SubtypeName = resource.SubtypeName("sensor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Servo's typed resource name.
func Named(name string) resource.Name {
	remotes := strings.Split(name, ":")
	if len(remotes) > 1 {
		rName := resource.NameFromSubtype(Subtype, remotes[len(remotes)-1])
		rName.PrependRemote(resource.RemoteName(strings.Join(remotes[:len(remotes)-1], ":")))
		return rName
	}
	return resource.NameFromSubtype(Subtype, name)
}

// A Sensor represents a general purpose sensors that can give arbitrary readings
// of some thing that it is sensing.
type Sensor interface {
	// GetReadings return data specific to the type of sensor and can be of any type.
	GetReadings(ctx context.Context) ([]interface{}, error)
	generic.Generic
}

var (
	_ = Sensor(&reconfigurableSensor{})
	_ = resource.Reconfigurable(&reconfigurableSensor{})
)

// FromRobot is a helper for getting the named Sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (Sensor, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Sensor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Sensor", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all sensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableSensor struct {
	mu     sync.RWMutex
	actual Sensor
}

func (r *reconfigurableSensor) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableSensor) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

// Do passes generic commands/data.
func (r *reconfigurableSensor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableSensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetReadings(ctx)
}

func (r *reconfigurableSensor) Reconfigure(ctx context.Context, newSensor resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newSensor.(*reconfigurableSensor)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newSensor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Sensor implementation to a reconfigurableSensor.
// If Sensor is already a reconfigurableSensor, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	Sensor, ok := r.(Sensor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Sensor", r)
	}
	if reconfigurable, ok := Sensor.(*reconfigurableSensor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableSensor{actual: Sensor}, nil
}

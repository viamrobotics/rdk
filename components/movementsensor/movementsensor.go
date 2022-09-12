// Package movementsensor defines the interfaces of a MovementSensor
package movementsensor

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	pb "go.viam.com/rdk/proto/api/component/movementsensor/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// Properties tells you what a MovementSensor supports.
type Properties pb.GetPropertiesResponse

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.MovementSensorService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterMovementSensorServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.MovementSensorService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})

	registerCollector("GetPosition", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		p, _, err := ms.GetPosition(ctx)
		return p, err
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "movement_sensor".
const SubtypeName = resource.SubtypeName("movement_sensor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named MovementSensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A MovementSensor reports information about the robot's direction, position and speed.
type MovementSensor interface {
	GetPosition(ctx context.Context) (*geo.Point, float64, error)                // (lat, long), altitide (mm)
	GetLinearVelocity(ctx context.Context) (r3.Vector, error)                    // mm / sec
	GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) // radians / sec
	GetCompassHeading(ctx context.Context) (float64, error)                      // [0->360)
	GetOrientation(ctx context.Context) (spatialmath.Orientation, error)
	GetProperties(ctx context.Context) (*Properties, error)
	GetAccuracy(ctx context.Context) (map[string]float32, error) // in mm
	generic.Generic
	sensor.Sensor
}

var (
	_ = MovementSensor(&reconfigurableMovementSensor{})
	_ = sensor.Sensor(&reconfigurableMovementSensor{})
	_ = resource.Reconfigurable(&reconfigurableMovementSensor{})
	_ = viamutils.ContextCloser(&reconfigurableMovementSensor{})
)

// FromDependencies is a helper for getting the named movementsensor from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (MovementSensor, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(MovementSensor)
	if !ok {
		return nil, utils.DependencyTypeError(name, "MovementSensor", res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named MovementSensor from the given Robot.
func FromRobot(r robot.Robot, name string) (MovementSensor, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(MovementSensor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("MovementSensor", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all MovementSensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// GetReadings is a helper for getting all readings from a MovementSensor.
func GetReadings(ctx context.Context, g MovementSensor) (map[string]interface{}, error) {
	readings := map[string]interface{}{}

	pos, altitide, err := g.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	readings["position"] = pos
	readings["altitide"] = altitide

	vel, err := g.GetLinearVelocity(ctx)
	if err != nil {
		return nil, err
	}
	readings["linear_velocity"] = vel

	avel, err := g.GetAngularVelocity(ctx)
	if err != nil {
		return nil, err
	}
	readings["angular_velocity"] = avel

	compass, err := g.GetCompassHeading(ctx)
	if err != nil {
		return nil, err
	}
	readings["compass"] = compass

	ori, err := g.GetOrientation(ctx)
	if err != nil {
		return nil, err
	}
	readings["orientation"] = ori

	return readings, nil
}

type reconfigurableMovementSensor struct {
	mu     sync.RWMutex
	actual MovementSensor
}

func (r *reconfigurableMovementSensor) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableMovementSensor) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableMovementSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.DoCommand(ctx, cmd)
}

func (r *reconfigurableMovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetPosition(ctx)
}

func (r *reconfigurableMovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetAngularVelocity(ctx)
}

func (r *reconfigurableMovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetLinearVelocity(ctx)
}

func (r *reconfigurableMovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetOrientation(ctx)
}

func (r *reconfigurableMovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetCompassHeading(ctx)
}

func (r *reconfigurableMovementSensor) GetProperties(ctx context.Context) (*Properties, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetProperties(ctx)
}

func (r *reconfigurableMovementSensor) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetAccuracy(ctx)
}

func (r *reconfigurableMovementSensor) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetReadings(ctx)
}

func (r *reconfigurableMovementSensor) Reconfigure(ctx context.Context, newMovementSensor resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reconfigure(ctx, newMovementSensor)
}

func (r *reconfigurableMovementSensor) reconfigure(ctx context.Context, newMovementSensor resource.Reconfigurable) error {
	actual, ok := newMovementSensor.(*reconfigurableMovementSensor)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newMovementSensor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable - if MovementSensor is already a reconfigurableMovementSensor, then nothing is done.
// Otherwise wraps in a Reconfigurable.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	ms, ok := r.(MovementSensor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("MovementSensor", r)
	}
	if reconfigurable, ok := ms.(*reconfigurableMovementSensor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableMovementSensor{actual: ms}, nil
}

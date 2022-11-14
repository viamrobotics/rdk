// Package movementsensor defines the interfaces of a MovementSensor
package movementsensor

import (
	"context"
	"errors"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/component/movementsensor/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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

	registerCollector("Position", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		type Position struct {
			Lat float64
			Lng float64
		}
		p, _, err := ms.Position(ctx, make(map[string]interface{}))
		return Position{Lat: p.Lat(), Lng: p.Lng()}, err
	})
	registerCollector("LinearVelocity", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		v, err := ms.LinearVelocity(ctx, make(map[string]interface{}))
		return v, err
	})
	registerCollector("AngularVelocity", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		v, err := ms.AngularVelocity(ctx, make(map[string]interface{}))
		return v, err
	})
	registerCollector("CompassHeading", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		type Heading struct {
			Heading float64
		}
		h, err := ms.CompassHeading(ctx, make(map[string]interface{}))
		return Heading{Heading: h}, err
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
	Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error)                // (lat, long), altitide (mm)
	LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)                    // mm / sec
	AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) // radians / sec
	CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error)                      // [0->360)
	Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error)
	Properties(ctx context.Context, extra map[string]interface{}) (*Properties, error)
	Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) // in mm
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
		return nil, DependencyTypeError(name, res)
	}
	return part, nil
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((MovementSensor)(nil), actual)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError(name, actual interface{}) error {
	return utils.DependencyTypeError(name, (MovementSensor)(nil), actual)
}

// FromRobot is a helper for getting the named MovementSensor from the given Robot.
func FromRobot(r robot.Robot, name string) (MovementSensor, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(MovementSensor)
	if !ok {
		return nil, NewUnimplementedInterfaceError(res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all MovementSensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// Readings is a helper for getting all readings from a MovementSensor.
func Readings(ctx context.Context, g MovementSensor, extra map[string]interface{}) (map[string]interface{}, error) {
	readings := map[string]interface{}{}

	pos, altitide, err := g.Position(ctx, extra)
	if err != nil && !errors.Is(err, ErrMethodUnimplementedPosition) {
		return nil, err
	}
	readings["position"] = pos
	readings["altitide"] = altitide

	vel, err := g.LinearVelocity(ctx, extra)
	if err != nil && !errors.Is(err, ErrMethodUnimplementedLinearVelocity) {
		return nil, err
	}
	readings["linear_velocity"] = vel

	avel, err := g.AngularVelocity(ctx, extra)
	if err != nil && !errors.Is(err, ErrMethodUnimplementedAngularVelocity) {
		return nil, err
	}
	readings["angular_velocity"] = avel

	compass, err := g.CompassHeading(ctx, extra)
	if err != nil && !errors.Is(err, ErrMethodUnimplementedCompassHeading) {
		return nil, err
	}
	readings["compass"] = compass

	ori, err := g.Orientation(ctx, extra)
	if err != nil && !errors.Is(err, ErrMethodUnimplementedOrientation) {
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

func (r *reconfigurableMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Position(ctx, extra)
}

func (r *reconfigurableMovementSensor) AngularVelocity(
	ctx context.Context,
	extra map[string]interface{},
) (spatialmath.AngularVelocity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.AngularVelocity(ctx, extra)
}

func (r *reconfigurableMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.LinearVelocity(ctx, extra)
}

func (r *reconfigurableMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Orientation(ctx, extra)
}

func (r *reconfigurableMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.CompassHeading(ctx, extra)
}

func (r *reconfigurableMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*Properties, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Properties(ctx, extra)
}

func (r *reconfigurableMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Accuracy(ctx, extra)
}

func (r *reconfigurableMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx, extra)
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
		golog.Global().Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable - if MovementSensor is already a reconfigurableMovementSensor, then nothing is done.
// Otherwise wraps in a Reconfigurable.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	ms, ok := r.(MovementSensor)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := ms.(*reconfigurableMovementSensor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableMovementSensor{actual: ms}, nil
}

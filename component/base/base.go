// Package base defines the base that a robot uses to move around.
package base

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/base/v1"
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
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			return CreateStatus(ctx, resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.BaseService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterBaseServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.BaseService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "base".
const SubtypeName = resource.SubtypeName("base")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Base's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Base represents a physical base of a robot.
type Base interface {
	// MoveStraight moves the robot straight a given distance at a given speed.
	// If a distance or speed of zero is given, the base will stop.
	// This method blocks until completed or cancelled
	MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error

	// Spin spins the robot by a given angle in degrees at a given speed.
	// If a speed of 0 the base will stop.
	// This method blocks until completed or cancelled
	Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error

	SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	// linear is in mmPerSec
	// angular is in degsPerSec
	SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	// Stop stops the base. It is assumed the base stops immediately.
	Stop(ctx context.Context, extra map[string]interface{}) error

	generic.Generic
}

// A LocalBase represents a physical base of a robot that can report the width of itself.
type LocalBase interface {
	Base
	// GetWidth returns the width of the base in millimeters.
	GetWidth(ctx context.Context) (int, error)
	resource.MovingCheckable
}

var (
	_ = Base(&reconfigurableBase{})
	_ = LocalBase(&reconfigurableLocalBase{})
	_ = resource.Reconfigurable(&reconfigurableBase{})
	_ = resource.Reconfigurable(&reconfigurableLocalBase{})
	_ = viamutils.ContextCloser(&reconfigurableLocalBase{})
)

// FromDependencies is a helper for getting the named base from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (Base, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(Base)
	if !ok {
		return nil, utils.DependencyTypeError(name, "Base", res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named base from the given Robot.
func FromRobot(r robot.Robot, name string) (Base, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Base)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Base", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all base names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the base.
func CreateStatus(ctx context.Context, resource interface{}) (*commonpb.ActuatorStatus, error) {
	base, ok := resource.(LocalBase)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalBase", resource)
	}
	isMoving, err := base.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &commonpb.ActuatorStatus{IsMoving: isMoving}, nil
}

type reconfigurableBase struct {
	mu     sync.RWMutex
	actual Base
}

func (r *reconfigurableBase) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableBase) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveStraight(ctx, distanceMm, mmPerSec, extra)
}

func (r *reconfigurableBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Spin(ctx, angleDeg, degsPerSec, extra)
}

func (r *reconfigurableBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.SetPower(ctx, linear, angular, extra)
}

func (r *reconfigurableBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.SetVelocity(ctx, linear, angular, extra)
}

func (r *reconfigurableBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx, extra)
}

func (r *reconfigurableBase) UpdateAction(c *config.Component) config.UpdateActionType {
	obj, canUpdate := r.actual.(config.ComponentUpdate)
	if canUpdate {
		return obj.UpdateAction(c)
	}
	return config.Reconfigure
}

func (r *reconfigurableBase) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableBase) Reconfigure(ctx context.Context, newBase resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reconfigure(ctx, newBase)
}

func (r *reconfigurableBase) reconfigure(ctx context.Context, newBase resource.Reconfigurable) error {
	actual, ok := newBase.(*reconfigurableBase)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newBase)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

type reconfigurableLocalBase struct {
	*reconfigurableBase
	actual LocalBase
}

func (r *reconfigurableLocalBase) GetWidth(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetWidth(ctx)
}

func (r *reconfigurableLocalBase) IsMoving(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsMoving(ctx)
}

func (r *reconfigurableLocalBase) Reconfigure(ctx context.Context, newBase resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newBase.(*reconfigurableLocalBase)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newBase)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}

	r.actual = actual.actual
	return r.reconfigurableBase.reconfigure(ctx, actual.reconfigurableBase)
}

// WrapWithReconfigurable converts a regular LocalBase implementation to a reconfigurableBase.
// If base is already a reconfigurableBase, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	base, ok := r.(Base)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Base", r)
	}
	if reconfigurable, ok := base.(*reconfigurableBase); ok {
		return reconfigurable, nil
	}

	rBase := &reconfigurableBase{actual: base}
	localBase, ok := r.(LocalBase)
	if !ok {
		return rBase, nil
	}

	if reconfigurable, ok := localBase.(*reconfigurableLocalBase); ok {
		return reconfigurable, nil
	}
	return &reconfigurableLocalBase{actual: localBase, reconfigurableBase: rBase}, nil
}

// A Move describes instructions for a robot to spin followed by moving straight.
type Move struct {
	DistanceMm int
	MmPerSec   float64
	AngleDeg   float64
	DegsPerSec float64
	Extra      map[string]interface{}
}

// DoMove performs the given move on the given base.
func DoMove(ctx context.Context, move Move, base Base) error {
	if move.AngleDeg != 0 {
		err := base.Spin(ctx, move.AngleDeg, move.DegsPerSec, move.Extra)
		if err != nil {
			return err
		}
	}

	if move.DistanceMm != 0 {
		err := base.MoveStraight(ctx, move.DistanceMm, move.MmPerSec, move.Extra)
		if err != nil {
			return err
		}
	}

	return nil
}

// Package base defines the base that a robot uses to move around.
package base

import (
	"context"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
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
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.BaseService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterBaseServiceHandlerFromEndpoint,
			)
		},
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
	remotes := strings.Split(name, ":")
	if len(remotes) > 1 {
		rName := resource.NameFromSubtype(Subtype, remotes[len(remotes)-1])
		rName.PrependRemote(resource.RemoteName(strings.Join(remotes[:len(remotes)-1], ":")))
		return rName
	}
	return resource.NameFromSubtype(Subtype, name)
}

// A Base represents a physical base of a robot.
type Base interface {
	// MoveStraight moves the robot straight a given distance at a given speed.
	// If a distance or speed of zero is given, the base will stop.
	// This method blocks until completed or cancelled
	MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error

	// Spin spins the robot by a given angle in degrees at a given speed.
	// If a speed of 0 the base will stop.
	// This method blocks until completed or cancelled
	Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error

	SetPower(ctx context.Context, linear, angular r3.Vector) error

	// linear is in mmPerSec
	// angular is in degsPerSec
	SetVelocity(ctx context.Context, linear, angular r3.Vector) error

	// Stop stops the base. It is assumed the base stops immediately.
	Stop(ctx context.Context) error

	generic.Generic
}

// A LocalBase represents a physical base of a robot that can report the width of itself.
type LocalBase interface {
	Base
	// GetWidth returns the width of the base in millimeters.
	GetWidth(ctx context.Context) (int, error)
}

var (
	_ = LocalBase(&reconfigurableBase{})
	_ = resource.Reconfigurable(&reconfigurableBase{})
)

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

type reconfigurableBase struct {
	mu     sync.RWMutex
	actual LocalBase
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
	ctx context.Context, distanceMm int, mmPerSec float64,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveStraight(ctx, distanceMm, mmPerSec)
}

func (r *reconfigurableBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Spin(ctx, angleDeg, degsPerSec)
}

func (r *reconfigurableBase) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.SetPower(ctx, linear, angular)
}

func (r *reconfigurableBase) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.SetVelocity(ctx, linear, angular)
}

func (r *reconfigurableBase) Stop(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx)
}

func (r *reconfigurableBase) GetWidth(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetWidth(ctx)
}

func (r *reconfigurableBase) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableBase) Reconfigure(ctx context.Context, newBase resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (r *reconfigurableBase) UpdateAction(c *config.Component) config.UpdateActionType {
	obj, canUpdate := r.actual.(config.CompononentUpdate)
	if canUpdate {
		return obj.UpdateAction(c)
	}
	return config.Reconfigure
}

// WrapWithReconfigurable converts a regular LocalBase implementation to a reconfigurableBase.
// If base is already a reconfigurableBase, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	base, ok := r.(LocalBase)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalBase", r)
	}
	if reconfigurable, ok := base.(*reconfigurableBase); ok {
		return reconfigurable, nil
	}
	return &reconfigurableBase{actual: base}, nil
}

// A Move describes instructions for a robot to spin followed by moving straight.
type Move struct {
	DistanceMm int
	MmPerSec   float64
	AngleDeg   float64
	DegsPerSec float64
}

// DoMove performs the given move on the given base.
func DoMove(ctx context.Context, move Move, base Base) error {
	if move.AngleDeg != 0 {
		err := base.Spin(ctx, move.AngleDeg, move.DegsPerSec)
		if err != nil {
			return err
		}
	}

	if move.DistanceMm != 0 {
		err := base.MoveStraight(ctx, move.DistanceMm, move.MmPerSec)
		if err != nil {
			return err
		}
	}

	return nil
}

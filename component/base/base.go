// Package base defines the base that a robot uses to move around.
package base

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
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
	return resource.NameFromSubtype(Subtype, name)
}

// A MinimalBase represents a physical base of a robot.
type MinimalBase interface {
	// MoveStraight moves the robot straight a given distance at a given speed. The method
	// can be requested to block until the move is complete. If a distance or speed of zero is given,
	// the base will stop.
	MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error

	// MoveArc moves the robot in an arc a given distance at a given speed and degs per second of movement.
	// The degs per sec represents the angular velocity the robot has during its movement. This function
	// can be requested to block until move is complete. If a distance of 0 is given the resultant motion
	// is a spin and if speed of 0 is given the base will stop.
	// Note: ramping affects when and how arc is performed, further improvements may be needed
	MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64, block bool) error

	// Spin spins the robot by a given angle in degrees at a given speed. The method can be requested
	// to block until the move is complete. If a speed of 0 the base will stop.
	Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error

	// Stop stops the base. It is assumed the base stops immediately.
	Stop(ctx context.Context) error
}

// A LocalBase represents a physical base of a robot that can report the width of itself.
type LocalBase interface {
	MinimalBase
	// GetWidth returns the width of the base in millimeters.
	GetWidth(ctx context.Context) (int, error)
}

// Base represents a fully implemented Base.
type Base interface {
	generic.Generic
	MinimalBase
}

var (
	_ = LocalBase(&reconfigurableBase{})
	_ = generic.Generic(&reconfigurableBase{})
	_ = resource.Reconfigurable(&reconfigurableBase{})
)

// FromRobot is a helper for getting the named base from the given Robot.
func FromRobot(r robot.Robot, name string) (Base, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	if full, ok := res.(Base); ok {
		return full, nil
	}
	if part, ok := res.(MinimalBase); ok {
		return &reconfigurableBase{actual: part}, nil
	}
	return nil, utils.NewUnimplementedInterfaceError("Base", res)
}

// NamesFromRobot is a helper for getting all base names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableBase struct {
	mu     sync.RWMutex
	actual MinimalBase
}

func (r *reconfigurableBase) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, block bool,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveStraight(ctx, distanceMm, mmPerSec, block)
}

func (r *reconfigurableBase) MoveArc(
	ctx context.Context, distanceMm int, mmPerSec float64, degAngle float64, block bool,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveArc(ctx, distanceMm, mmPerSec, degAngle, block)
}

func (r *reconfigurableBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Spin(ctx, angleDeg, degsPerSec, block)
}

func (r *reconfigurableBase) Stop(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx)
}

func (r *reconfigurableBase) GetWidth(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if dev, ok := r.actual.(LocalBase); ok {
		return dev.GetWidth(ctx)
	}
	return -1, utils.NewUnimplementedInterfaceError("LocalBase", r)
}

func (r *reconfigurableBase) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

// Do will try to Do() and error if not implemented.
func (r *reconfigurableBase) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if dev, ok := r.actual.(generic.Generic); ok {
		return dev.Do(ctx, cmd)
	}
	return nil, utils.NewUnimplementedInterfaceError("Generic", r)
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

// WrapWithReconfigurable converts a regular LocalBase implementation to a reconfigurableBase.
// If base is already a reconfigurableBase, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	base, ok := r.(MinimalBase)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("MinimalBase", r)
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
	Block      bool
}

// DoMove performs the given move on the given base.
func DoMove(ctx context.Context, move Move, base MinimalBase) error {
	if move.AngleDeg != 0 {
		err := base.Spin(ctx, move.AngleDeg, move.DegsPerSec, move.Block)
		if err != nil {
			return err
		}
	}

	if move.DistanceMm != 0 {
		err := base.MoveStraight(ctx, move.DistanceMm, move.MmPerSec, move.Block)
		if err != nil {
			return err
		}
	}

	return nil
}

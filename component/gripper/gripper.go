// Package gripper defines a robotic gripper.
package gripper

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/gripper/v1"
	"go.viam.com/rdk/referenceframe"
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
				&pb.GripperService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGripperServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string.
const SubtypeName = resource.SubtypeName("gripper")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named grippers's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Gripper represents a physical robotic gripper.
type Gripper interface {
	// Open opens the gripper.
	// This will block until done or a new operation cancels this one
	Open(ctx context.Context) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	// This will block until done or a new operation cancels this one
	Grab(ctx context.Context) (bool, error)

	generic.Generic
	referenceframe.ModelFramer
}

// WrapWithReconfigurable wraps a gripper with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	g, ok := r.(Gripper)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Gripper", r)
	}
	if reconfigurable, ok := g.(*reconfigurableGripper); ok {
		return reconfigurable, nil
	}
	return &reconfigurableGripper{actual: g}, nil
}

var (
	_ = Gripper(&reconfigurableGripper{})
	_ = resource.Reconfigurable(&reconfigurableGripper{})
)

// FromRobot is a helper for getting the named Gripper from the given Robot.
func FromRobot(r robot.Robot, name string) (Gripper, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Gripper)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Gripper", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableGripper struct {
	mu     sync.RWMutex
	actual Gripper
}

func (g *reconfigurableGripper) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
}

func (g *reconfigurableGripper) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Do(ctx, cmd)
}

func (g *reconfigurableGripper) Open(ctx context.Context) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Open(ctx)
}

func (g *reconfigurableGripper) Grab(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Grab(ctx)
}

// Reconfigure reconfigures the resource.
func (g *reconfigurableGripper) Reconfigure(ctx context.Context, newGripper resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	actual, ok := newGripper.(*reconfigurableGripper)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newGripper)
	}
	if err := viamutils.TryClose(ctx, g.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	g.actual = actual.actual
	return nil
}

func (g *reconfigurableGripper) ModelFrame() referenceframe.Model {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.ModelFrame()
}

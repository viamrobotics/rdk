// Package gripper defines a robotic gripper.
package gripper

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
				&pb.GripperService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGripperServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.GripperService_ServiceDesc,
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
	Open(ctx context.Context, extra map[string]interface{}) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	// This will block until done or a new operation cancels this one
	Grab(ctx context.Context, extra map[string]interface{}) (bool, error)

	// Stop stops the gripper. It is assumed the gripper stops immediately.
	Stop(ctx context.Context, extra map[string]interface{}) error

	generic.Generic
	referenceframe.ModelFramer
}

// A LocalGripper represents a Gripper that can report whether it is moving or not.
type LocalGripper interface {
	Gripper

	resource.MovingCheckable
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Gripper)(nil), actual)
}

// NewUnimplementedLocalInterfaceError is used when there is a failed interface check.
func NewUnimplementedLocalInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((LocalGripper)(nil), actual)
}

// WrapWithReconfigurable wraps a gripper with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	g, ok := r.(Gripper)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := g.(*reconfigurableGripper); ok {
		return reconfigurable, nil
	}
	rGripper := &reconfigurableGripper{name: name, actual: g}
	gLocal, ok := r.(LocalGripper)
	if !ok {
		return rGripper, nil
	}
	if reconfigurable, ok := g.(*reconfigurableLocalGripper); ok {
		return reconfigurable, nil
	}

	return &reconfigurableLocalGripper{actual: gLocal, reconfigurableGripper: rGripper}, nil
}

var (
	_ = Gripper(&reconfigurableGripper{})
	_ = LocalGripper(&reconfigurableLocalGripper{})
	_ = resource.Reconfigurable(&reconfigurableGripper{})
	_ = resource.Reconfigurable(&reconfigurableLocalGripper{})

	// ErrStopUnimplemented is used for when Stop() is unimplemented.
	ErrStopUnimplemented = errors.New("Stop() unimplemented")
)

// FromRobot is a helper for getting the named Gripper from the given Robot.
func FromRobot(r robot.Robot, name string) (Gripper, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Gripper)
	if !ok {
		return nil, NewUnimplementedInterfaceError(res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the gripper.
func CreateStatus(ctx context.Context, resource interface{}) (*commonpb.ActuatorStatus, error) {
	gripper, ok := resource.(LocalGripper)
	if !ok {
		return nil, NewUnimplementedLocalInterfaceError(resource)
	}
	isMoving, err := gripper.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &commonpb.ActuatorStatus{IsMoving: isMoving}, nil
}

type reconfigurableGripper struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Gripper
}

func (g *reconfigurableGripper) Name() resource.Name {
	return g.name
}

func (g *reconfigurableGripper) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
}

func (g *reconfigurableGripper) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.DoCommand(ctx, cmd)
}

func (g *reconfigurableGripper) Open(ctx context.Context, extra map[string]interface{}) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Open(ctx, extra)
}

func (g *reconfigurableGripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Grab(ctx, extra)
}

func (g *reconfigurableGripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Stop(ctx, extra)
}

// Reconfigure reconfigures the resource.
func (g *reconfigurableGripper) Reconfigure(ctx context.Context, newGripper resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reconfigure(ctx, newGripper)
}

func (g *reconfigurableGripper) reconfigure(ctx context.Context, newGripper resource.Reconfigurable) error {
	actual, ok := newGripper.(*reconfigurableGripper)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newGripper)
	}
	if err := viamutils.TryClose(ctx, g.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	g.actual = actual.actual
	return nil
}

func (g *reconfigurableGripper) ModelFrame() referenceframe.Model {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.ModelFrame()
}

type reconfigurableLocalGripper struct {
	*reconfigurableGripper
	actual LocalGripper
}

func (g *reconfigurableLocalGripper) IsMoving(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.IsMoving(ctx)
}

func (g *reconfigurableLocalGripper) Reconfigure(ctx context.Context, newGripper resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	gripper, ok := newGripper.(*reconfigurableLocalGripper)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newGripper)
	}
	if err := viamutils.TryClose(ctx, g.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}

	g.actual = gripper.actual
	return g.reconfigurableGripper.reconfigure(ctx, gripper.reconfigurableGripper)
}

package gantry

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/data"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/gantry/v1"
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
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			return CreateStatus(ctx, resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.GantryService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGantryServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.GantryService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: getPosition.String(),
	}, newGetPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: getLengths.String(),
	}, newGetLengthsCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "gantry".
const SubtypeName = resource.SubtypeName("gantry")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Gantry's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// Gantry is used for controlling gantries of N axis.
type Gantry interface {
	// GetPosition returns the position in meters
	GetPosition(ctx context.Context, extra map[string]interface{}) ([]float64, error)

	// MoveToPosition is in meters
	// The worldState argument should be treated as optional by all implementing drivers
	// This will block until done or a new operation cancels this one
	MoveToPosition(ctx context.Context, positionsMm []float64, worldState *commonpb.WorldState, extra map[string]interface{}) error

	// GetLengths is the length of gantries in meters
	GetLengths(ctx context.Context, extra map[string]interface{}) ([]float64, error)

	// Stop stops the gantry. It is assumed the gantry stops immediately.
	Stop(ctx context.Context, extra map[string]interface{}) error

	generic.Generic
	referenceframe.ModelFramer
	referenceframe.InputEnabled
}

// FromDependencies is a helper for getting the named gantry from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (Gantry, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(Gantry)
	if !ok {
		return nil, utils.DependencyTypeError(name, "Gantry", res)
	}
	return part, nil
}

// A LocalGantry represents a Gantry that can report whether it is moving or not.
type LocalGantry interface {
	Gantry

	resource.MovingCheckable
}

// FromRobot is a helper for getting the named gantry from the given Robot.
func FromRobot(r robot.Robot, name string) (Gantry, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Gantry)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Gantry", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all gantry names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the gantry.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	gantry, ok := resource.(LocalGantry)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalGantry", resource)
	}
	positions, err := gantry.GetPosition(ctx, nil)
	if err != nil {
		return nil, err
	}

	lengths, err := gantry.GetLengths(ctx, nil)
	if err != nil {
		return nil, err
	}
	isMoving, err := gantry.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{PositionsMm: positions, LengthsMm: lengths, IsMoving: isMoving}, nil
}

// WrapWithReconfigurable wraps a gantry or localGantry with a reconfigurable
// and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	g, ok := r.(Gantry)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Gantry", r)
	}
	if reconfigurable, ok := g.(*reconfigurableGantry); ok {
		return reconfigurable, nil
	}

	rGantry := &reconfigurableGantry{actual: g}
	gLocal, ok := r.(LocalGantry)
	if !ok {
		return rGantry, nil
	}
	if reconfigurable, ok := gLocal.(*reconfigurableLocalGantry); ok {
		return reconfigurable, nil
	}
	return &reconfigurableLocalGantry{actual: gLocal, reconfigurableGantry: rGantry}, nil
}

var (
	_ = Gantry(&reconfigurableGantry{})
	_ = LocalGantry(&reconfigurableLocalGantry{})
	_ = resource.Reconfigurable(&reconfigurableGantry{})
	_ = resource.Reconfigurable(&reconfigurableLocalGantry{})
)

type reconfigurableGantry struct {
	mu     sync.RWMutex
	actual Gantry
}

func (g *reconfigurableGantry) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
}

func (g *reconfigurableGantry) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Do(ctx, cmd)
}

// GetPosition returns the position in meters.
func (g *reconfigurableGantry) GetPosition(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.GetPosition(ctx, extra)
}

// GetLengths returns the position in meters.
func (g *reconfigurableGantry) GetLengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.GetLengths(ctx, extra)
}

// position is in meters.
func (g *reconfigurableGantry) MoveToPosition(
	ctx context.Context,
	positionsMm []float64,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.MoveToPosition(ctx, positionsMm, worldState, extra)
}

func (g *reconfigurableGantry) Stop(ctx context.Context, extra map[string]interface{}) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Stop(ctx, extra)
}

func (g *reconfigurableGantry) Close(ctx context.Context) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return viamutils.TryClose(ctx, g.actual)
}

// Reconfigure reconfigures the resource.
func (g *reconfigurableGantry) Reconfigure(ctx context.Context, newGantry resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reconfigure(ctx, newGantry)
}

func (g *reconfigurableGantry) reconfigure(ctx context.Context, newGantry resource.Reconfigurable) error {
	actual, ok := newGantry.(*reconfigurableGantry)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newGantry)
	}
	if err := viamutils.TryClose(ctx, g.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	g.actual = actual.actual
	return nil
}

func (g *reconfigurableGantry) ModelFrame() referenceframe.Model {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.ModelFrame()
}

func (g *reconfigurableGantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.CurrentInputs(ctx)
}

func (g *reconfigurableGantry) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.GoToInputs(ctx, goal)
}

type reconfigurableLocalGantry struct {
	*reconfigurableGantry
	actual LocalGantry
}

func (g *reconfigurableLocalGantry) IsMoving(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.IsMoving(ctx)
}

func (g *reconfigurableLocalGantry) Reconfigure(ctx context.Context, newGantry resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	gantry, ok := newGantry.(*reconfigurableLocalGantry)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newGantry)
	}
	if err := viamutils.TryClose(ctx, g.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}

	g.actual = gantry.actual
	return g.reconfigurableGantry.reconfigure(ctx, gantry.reconfigurableGantry)
}

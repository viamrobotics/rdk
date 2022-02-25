package gantry

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

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
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.GantryService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGantryServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
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
	GetPosition(ctx context.Context) ([]float64, error)

	// MoveToPosition is in meters
	MoveToPosition(ctx context.Context, positionsMm []float64) error

	// GetLengths is the length of gantries in meters
	GetLengths(ctx context.Context) ([]float64, error)

	referenceframe.ModelFramer
	referenceframe.InputEnabled
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

// WrapWithReconfigurable wraps a gantry with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	g, ok := r.(Gantry)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Gantry", r)
	}
	if reconfigurable, ok := g.(*reconfigurableGantry); ok {
		return reconfigurable, nil
	}
	return &reconfigurableGantry{actual: g}, nil
}

type reconfigurableGantry struct {
	mu     sync.RWMutex
	actual Gantry
}

func (g *reconfigurableGantry) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
}

// GetPosition returns the position in meters.
func (g *reconfigurableGantry) GetPosition(ctx context.Context) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.GetPosition(ctx)
}

// GetLengths returns the position in meters.
func (g *reconfigurableGantry) GetLengths(ctx context.Context) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.GetLengths(ctx)
}

// position is in meters.
func (g *reconfigurableGantry) MoveToPosition(ctx context.Context, positionsMm []float64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.MoveToPosition(ctx, positionsMm)
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

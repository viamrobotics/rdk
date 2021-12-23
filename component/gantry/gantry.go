package gantry

import (
	"context"
	"sync"

	viamutils "go.viam.com/utils"

	"github.com/pkg/errors"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
)

// SubtypeName is a constant that identifies the component resource subtype string "gantry"
const SubtypeName = resource.SubtypeName("gantry")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Gantry's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

// Gantry is used for controlling gantries of N axis
type Gantry interface {
	// CurrentPosition returns the position in meters
	CurrentPosition(ctx context.Context) ([]float64, error)

	// MoveToPosition is in meters
	MoveToPosition(ctx context.Context, positions []float64) error

	// Lengths is the length of gantries in meters
	Lengths(ctx context.Context) ([]float64, error)

	referenceframe.ModelFramer
	referenceframe.InputEnabled
}

// WrapWithReconfigurable wraps a gantry with a reconfigurable and locking interface
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	g, ok := r.(Gantry)
	if !ok {
		return nil, errors.Errorf("expected resource to be Gantry but got %T", r)
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

// CurrentPosition returns the position in meters
func (g *reconfigurableGantry) CurrentPosition(ctx context.Context) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.CurrentPosition(ctx)
}

// Lengths returns the position in meters
func (g *reconfigurableGantry) Lengths(ctx context.Context) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.Lengths(ctx)
}

// position is in meters
func (g *reconfigurableGantry) MoveToPosition(ctx context.Context, positions []float64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.actual.MoveToPosition(ctx, positions)
}

// Reconfigure reconfigures the resource
func (g *reconfigurableGantry) Reconfigure(newGantry resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	actual, ok := newGantry.(*reconfigurableGantry)
	if !ok {
		return errors.Errorf("expected new gantry to be %T but got %T", g, newGantry)
	}
	if err := viamutils.TryClose(g.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	g.actual = actual.actual
	return nil
}

func (g *reconfigurableGantry) ModelFrame() *referenceframe.Model {
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

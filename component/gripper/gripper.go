// Package gripper defines a robotic gripper.
package gripper

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
)

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
	return resource.NewFromSubtype(Subtype, name)
}

// A Gripper represents a physical robotic gripper.
type Gripper interface {
	// Open opens the gripper.
	Open(ctx context.Context) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	Grab(ctx context.Context) (bool, error)

	referenceframe.ModelFramer
}

// WrapWithReconfigurable wraps a gripper with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	g, ok := r.(Gripper)
	if !ok {
		return nil, errors.Errorf("expected resource to be Gripper but got %T", r)
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

type reconfigurableGripper struct {
	mu     sync.RWMutex
	actual Gripper
}

func (g *reconfigurableGripper) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
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
		return errors.Errorf("expected new gripper to be %T but got %T", g, newGripper)
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

package gizmoapi

// The contents of this file are only needed for standalone (non-module) uses.

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func wrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	mc, ok := r.(Gizmo)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := mc.(*reconfigurableGizmo); ok {
		return reconfigurable, nil
	}
	return &reconfigurableGizmo{actual: mc, name: name}, nil
}

var (
	_ = Gizmo(&reconfigurableGizmo{})
	_ = resource.Reconfigurable(&reconfigurableGizmo{})
)

type reconfigurableGizmo struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Gizmo
}

func (g *reconfigurableGizmo) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
}

func (g *reconfigurableGizmo) DoOne(ctx context.Context, arg1 string) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.DoOne(ctx, arg1)
}

func (g *reconfigurableGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.DoOneClientStream(ctx, arg1)
}

func (g *reconfigurableGizmo) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.DoOneServerStream(ctx, arg1)
}

func (g *reconfigurableGizmo) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.DoOneBiDiStream(ctx, arg1)
}

func (g *reconfigurableGizmo) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.DoTwo(ctx, arg1)
}

func (g *reconfigurableGizmo) Reconfigure(ctx context.Context, newGizmo resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	actual, ok := newGizmo.(*reconfigurableGizmo)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newGizmo)
	}
	if err := goutils.TryClose(ctx, g.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	g.actual = actual.actual
	return nil
}

func (g *reconfigurableGizmo) Name() resource.Name {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.name
}

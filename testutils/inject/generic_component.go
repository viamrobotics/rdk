// Package inject provides dependency injected structures for mocking interfaces.
package inject

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// GenericComponent is an injectable generic component.
type GenericComponent struct {
	resource.Resource
	name           resource.Name
	DoFunc         func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc     func(ctx context.Context) (map[string]interface{}, error)
	CloseFunc      func(ctx context.Context) error
	GeometriesFunc func(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error)
}

// NewGenericComponent returns a new injected generic component.
func NewGenericComponent(name string) *GenericComponent {
	return &GenericComponent{name: generic.Named(name)}
}

// Name returns the name of the resource.
func (g *GenericComponent) Name() resource.Name {
	return g.name
}

// DoCommand calls the injected DoCommand or the real version.
func (g *GenericComponent) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.Resource.DoCommand(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (g *GenericComponent) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		if g.Resource == nil {
			return nil
		}
		return g.Resource.Close(ctx)
	}
	return g.CloseFunc(ctx)
}

// Status calls the injected Status or the real version.
func (g *GenericComponent) Status(ctx context.Context) (map[string]interface{}, error) {
	if g.StatusFunc != nil {
		return g.StatusFunc(ctx)
	}
	if g.Resource != nil {
		return g.Resource.Status(ctx)
	}
	return map[string]interface{}{}, nil
}

// Geometries calls the injected GeometriesFunc or the embedded resource's Geometries if it implements [resource.Shaped].
func (g *GenericComponent) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	if g.GeometriesFunc != nil {
		return g.GeometriesFunc(ctx, extra)
	}
	if shaped, ok := g.Resource.(resource.Shaped); ok {
		return shaped.Geometries(ctx, extra)
	}
	return nil, nil
}

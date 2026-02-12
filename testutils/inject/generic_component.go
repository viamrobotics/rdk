// Package inject provides dependency injected structures for mocking interfaces.
//
//nolint:dupl
package inject

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
)

// GenericComponent is an injectable generic component.
type GenericComponent struct {
	resource.Resource
	name      resource.Name
	DoFunc    func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
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

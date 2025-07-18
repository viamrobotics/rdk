// Package inject provides dependency injected structures for mocking interfaces.
//
//nolint:dupl
package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
)

// GenericService is an injectable generic service.
type GenericService struct {
	resource.Resource
	name      resource.Name
	DoFunc    func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewGenericService returns a new injected generic service.
func NewGenericService(name string) *GenericService {
	return &GenericService{name: generic.Named(name)}
}

// Name returns the name of the resource.
func (g *GenericService) Name() resource.Name {
	return g.name
}

// DoCommand calls the injected DoCommand or the real version.
func (g *GenericService) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.Resource.DoCommand(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (g *GenericService) Close(ctx context.Context) error {
	if g.CloseFunc == nil {
		if g.Resource == nil {
			return nil
		}
		return g.Resource.Close(ctx)
	}
	return g.CloseFunc(ctx)
}

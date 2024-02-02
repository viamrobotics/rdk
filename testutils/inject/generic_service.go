package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
)

// GenericService is an injectable generic service.
type GenericService struct {
	resource.Resource
	name   resource.Name
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
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

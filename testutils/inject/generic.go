package inject

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
)

// Generic is an injectable Generic component.
type Generic struct {
	resource.Resource
	name   resource.Name
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// NewGeneric returns a new injected generic.
func NewGeneric(name string) *Generic {
	return &Generic{name: generic.Named(name)}
}

// Name returns the name of the resource.
func (g *Generic) Name() resource.Name {
	return g.name
}

// DoCommand calls the injected DoCommand or the real version.
func (g *Generic) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.Resource.DoCommand(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

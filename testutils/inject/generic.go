package inject

import (
	"context"

	"go.viam.com/rdk/component/generic"
)

// Generic is an injectable Generic component.
type Generic struct {
	generic.Generic
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Do calls the injected Do or the real version.
func (g *Generic) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.Generic.Do(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

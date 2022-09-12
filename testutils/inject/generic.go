package inject

import (
	"context"

	"go.viam.com/rdk/components/generic"
)

// Generic is an injectable Generic component.
type Generic struct {
	generic.Generic
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// DoCommand calls the injected DoCommand or the real version.
func (g *Generic) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if g.DoFunc == nil {
		return g.Generic.DoCommand(ctx, cmd)
	}
	return g.DoFunc(ctx, cmd)
}

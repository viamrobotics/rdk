package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/metadata"
)

// Metadata is an injected metadata.
type Metadata struct {
	metadata.Service
	ResourcesFunc func() []resource.Name
}

// Resources calls the injected Resources or the real version.
func (m *Metadata) Resources(ctx context.Context) []resource.Name {
	if m.ResourcesFunc == nil {
		return m.Resources(ctx)
	}
	return m.ResourcesFunc()
}

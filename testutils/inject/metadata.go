package inject

import (
	"go.viam.com/core/metadata/service"
	"go.viam.com/core/resource"
)

// Metadata is an injected metadata.
type Metadata struct {
	service.Service
	AllFunc func() []resource.Name
}

// All calls the injected All or the real version.
func (m *Metadata) All() []resource.Name {
	if m.AllFunc == nil {
		return m.All()
	}
	return m.AllFunc()
}

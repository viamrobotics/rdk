// Package fake implements a fake Generic component.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterComponent(
		generic.Subtype,
		resource.NewDefaultModel("fake"),
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return newGeneric(conf.ResourceName()), nil
		}})
}

func newGeneric(name resource.Name) resource.Resource {
	return &Generic{Named: name.AsNamed()}
}

// Generic is a fake Generic device that always echos inputs back to the caller.
type Generic struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

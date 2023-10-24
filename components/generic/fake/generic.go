// Package fake implements a fake Generic component.
package fake

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterComponent(
		generic.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return newGeneric(conf.ResourceName(), logging.FromZapCompatible(logger)), nil
		}})
}

func newGeneric(name resource.Name, logger logging.Logger) resource.Resource {
	return &Generic{Named: name.AsNamed(), logger: logger}
}

// Generic is a fake Generic device that always echos inputs back to the caller.
type Generic struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

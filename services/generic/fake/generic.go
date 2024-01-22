// Package fake implements a fake generic service.
package fake

import (
	"context"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
)

func init() {
	resource.RegisterService(
		generic.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return newGeneric(conf.ResourceName(), logger), nil
		}})
}

func newGeneric(name resource.Name, logger logging.Logger) resource.Resource {
	return &Generic{Named: name.AsNamed(), logger: logger}
}

// Generic is a fake Generic service that always echos input back to the caller.
type Generic struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

// DoCommand echos input back to the caller.
func (fg *Generic) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

// Package fake implements a fake Generic component.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

func init() {
	registry.RegisterComponent(
		generic.Subtype,
		resource.NewDefaultModel("fake"),
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newGeneric(config.Name), nil
		}})
}

func newGeneric(name string) generic.Generic {
	return &Generic{Name: name}
}

// Generic is a fake Generic device that always echos inputs back to the caller.
type Generic struct {
	Name string
	generic.Echo
}

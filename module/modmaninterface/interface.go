// Package modmaninterface abstracts the manager interface to avoid an import cycle/loop.
package modmaninterface

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

// ModuleManager abstracts the module manager interface.
type ModuleManager interface {
	Add(ctx context.Context, cfg config.Module) error

	AddResource(ctx context.Context, cfg config.Component, deps []string) (interface{}, error)
	ReconfigureResource(ctx context.Context, cfg config.Component, deps []string) error
	RemoveResource(ctx context.Context, name resource.Name) error
	IsModularResource(name resource.Name) bool
	ValidateConfig(ctx context.Context, cfg config.Component) ([]string, error)

	Provides(cfg config.Component) bool

	Close(ctx context.Context) error
}

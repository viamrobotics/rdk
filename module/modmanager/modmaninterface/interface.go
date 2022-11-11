// Package modmaninterface abstracts the manager interface to avoid an import cycle/loop.
package modmaninterface

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

// ModuleManager abstracts the module manager interface.
type ModuleManager interface {
	AddModule(ctx context.Context, cfg config.Module) error

	AddComponent(ctx context.Context, cfg config.Component, deps []string) (interface{}, error)
	ReconfigureComponent(ctx context.Context, cfg config.Component, deps []string) error
	IsModularComponent(cfg config.Component) bool

	AddService(ctx context.Context, cfg config.Service, deps []string) (interface{}, error)
	ReconfigureService(ctx context.Context, cfg config.Service) error
	IsModularService(cfg config.Service) bool

	IsModularResource(name resource.Name) bool
	RemoveResource(ctx context.Context, name resource.Name) error

	Close(ctx context.Context) error
}

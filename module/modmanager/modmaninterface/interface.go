// Package modmaninterface abstracts the manager interface to avoid an import cycle/loop.
package modmaninterface

import (
	"context"

	"go.viam.com/rdk/config"
)

// ModuleManager abstracts the module manager interface.
type ModuleManager interface {
	AddModule(ctx context.Context, cfg config.Module) error
	AddComponent(ctx context.Context, cfg config.Component, deps []string) (interface{}, error)
	ReconfigureComponent(ctx context.Context, cfg config.Component, deps []string) error
	AddService(ctx context.Context, cfg config.Service) (interface{}, error)
	ReconfigureService(ctx context.Context, cfg config.Service) error

	IsModularComponent(cfg config.Component) bool
	IsModularService(cfg config.Service) bool

	Close(ctx context.Context) error
}

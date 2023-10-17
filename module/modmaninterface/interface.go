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
	Reconfigure(ctx context.Context, cfg config.Module) ([]resource.Name, error)
	Remove(modName string) ([]resource.Name, error)

	AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error)
	ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error
	RemoveResource(ctx context.Context, name resource.Name) error
	IsModularResource(name resource.Name) bool
	ValidateConfig(ctx context.Context, cfg resource.Config) ([]string, error)
    CleanModuleDataDirectory(ctx context.Context) error

	Configs() []config.Module
	Provides(cfg resource.Config) bool

	Close(ctx context.Context) error
}

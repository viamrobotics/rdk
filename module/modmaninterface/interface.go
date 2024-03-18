// Package modmaninterface abstracts the manager interface to avoid an import cycle/loop.
package modmaninterface

import (
	"context"

	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var Managers = []ModuleManager{}

// ModuleManager abstracts the module manager interface.
type ModuleManager interface {
	Add(ctx context.Context, confs ...config.Module) error
	Reconfigure(ctx context.Context, cfg config.Module) ([]resource.Name, error)
	Remove(modName string) ([]resource.Name, error)
	AddStream(ctx context.Context, name string) (*streampb.AddStreamResponse, error)
	AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error)
	ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error
	RemoveResource(ctx context.Context, name resource.Name) error
	IsModularResource(name resource.Name) bool
	ValidateConfig(ctx context.Context, cfg resource.Config) ([]string, error)
	ResolveImplicitDependenciesInConfig(ctx context.Context, conf *config.Diff) error
	CleanModuleDataDirectory() error

	Configs() []config.Module
	Provides(cfg resource.Config) bool
	Handles() map[string]module.HandlerMap

	Close(ctx context.Context) error
}

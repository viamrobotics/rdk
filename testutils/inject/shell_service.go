package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/utils"
)

// ShellService represents a fake instance of a shell service.
type ShellService struct {
	shell.Service
	name          resource.Name
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
	ReconfigureFunc func(ctx context.Context, deps resource.Dependencies, conf resource.Config) error
	CloseFunc       func(ctx context.Context) error
}

// NewShellService returns a new injected shell service.
func NewShellService(name string) *ShellService {
	return &ShellService{name: shell.Named(name)}
}

// Name returns the name of the resource.
func (s *ShellService) Name() resource.Name {
	return s.name
}

// DoCommand calls the injected DoCommand or the real variant.
func (s *ShellService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if s.DoCommandFunc == nil {
		return s.Service.DoCommand(ctx, cmd)
	}
	return s.DoCommandFunc(ctx, cmd)
}

// Reconfigure calls the injected Reconfigure or the real variant.
func (s *ShellService) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	if s.ReconfigureFunc == nil {
		if s.Service == nil {
			return resource.NewMustRebuildError(conf.ResourceName())
		}
		return s.Service.Reconfigure(ctx, deps, conf)
	}
	return s.ReconfigureFunc(ctx, deps, conf)
}

// Close calls the injected Close or the real version.
func (s *ShellService) Close(ctx context.Context) error {
	if s.CloseFunc == nil {
		return utils.TryClose(ctx, s.Service)
	}
	return s.CloseFunc(ctx)
}

package inject

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/services/shell"
)

// ShellService represents a fake instance of a shell service.
type ShellService struct {
	shell.Service
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
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

// ShellServiceWithReconfigure represents a fake instance of a shell service.
type ShellServiceWithReconfigure struct {
	shell.Service
	DoCommandFunc   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ReconfigureFunc func(ctx context.Context, cfg config.Service, deps registry.Dependencies) error
}

// DoCommand calls the injected DoCommand or the real variant.
func (s *ShellServiceWithReconfigure) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if s.DoCommandFunc == nil {
		return s.Service.DoCommand(ctx, cmd)
	}
	return s.DoCommandFunc(ctx, cmd)
}

// Reconfigure calls the injected Reconfigure or the real variant.
func (s *ShellServiceWithReconfigure) Reconfigure(ctx context.Context, cfg config.Service, deps registry.Dependencies) error {
	if s.ReconfigureFunc == nil {
		reconf, ok := s.Service.(registry.ReconfigurableService)
		if ok {
			return reconf.Reconfigure(ctx, cfg, deps)
		}
		return errors.New("no reconfigure function set")
	}
	return s.ReconfigureFunc(ctx, cfg, deps)
}

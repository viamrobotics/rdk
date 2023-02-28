package inject

import (
	"context"

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

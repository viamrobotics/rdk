package inject

import (
	"context"

	"go.viam.com/rdk/services/datamanager"
)

// DataManagerService represents a fake instance of an data manager
// service.
type DataManagerService struct {
	datamanager.Service
	SyncFunc      func(ctx context.Context, extra map[string]interface{}) error
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
}

// Sync calls the injected Sync or the real variant.
func (svc *DataManagerService) Sync(ctx context.Context, extra map[string]interface{}) error {
	if svc.SyncFunc == nil {
		return svc.Service.Sync(ctx, extra)
	}
	return svc.SyncFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (svc *DataManagerService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if svc.DoCommandFunc == nil {
		return svc.Service.DoCommand(ctx, cmd)
	}
	return svc.DoCommandFunc(ctx, cmd)
}

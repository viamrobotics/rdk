package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

// DataManagerService represents a fake instance of an data manager
// service.
type DataManagerService struct {
	datamanager.Service
	SyncFunc func(
		ctx context.Context,
		componentName resource.Name,
	) (bool, error)
}

// Move calls the injected Move or the real variant.
func (svc *DataManagerService) Sync(
	ctx context.Context,
	componentName resource.Name,
) (bool, error) {
	if svc.SyncFunc == nil {
		return svc.Service.Sync(ctx, componentName)
	}
	return svc.SyncFunc(ctx, componentName)
}

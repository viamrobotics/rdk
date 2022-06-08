package inject

import (
	"context"

	"go.viam.com/rdk/services/datamanager"
)

// DataManagerService represents a fake instance of an data manager
// service.
type DataManagerService struct {
	datamanager.Service
	SyncFunc func(
		ctx context.Context,
	) error
}

// Move calls the injected Move or the real variant.
func (svc *DataManagerService) Sync(
	ctx context.Context,
) error {
	if svc.SyncFunc == nil {
		return svc.Service.Sync(ctx)
	}
	return svc.SyncFunc(ctx)
}

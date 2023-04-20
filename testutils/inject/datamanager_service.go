package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/utils"
)

// DataManagerService represents a fake instance of an data manager
// service.
type DataManagerService struct {
	datamanager.Service
	name          resource.Name
	SyncFunc      func(ctx context.Context, extra map[string]interface{}) error
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewDataManagerService returns a new injected data manager service.
func NewDataManagerService(name string) *DataManagerService {
	return &DataManagerService{name: datamanager.Named(name)}
}

// Name returns the name of the resource.
func (svc *DataManagerService) Name() resource.Name {
	return svc.name
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

// Close calls the injected Close or the real version.
func (svc *DataManagerService) Close(ctx context.Context) error {
	if svc.CloseFunc == nil {
		return utils.TryClose(ctx, svc.Service)
	}
	return svc.CloseFunc(ctx)
}

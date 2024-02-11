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
	name          resource.Name
	SyncFunc      func(ctx context.Context, extra map[string]any) error
	DoCommandFunc func(ctx context.Context,
		cmd map[string]any) (map[string]any, error)
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
func (svc *DataManagerService) Sync(ctx context.Context, extra map[string]any) error {
	if svc.SyncFunc == nil {
		return svc.Service.Sync(ctx, extra)
	}
	return svc.SyncFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (svc *DataManagerService) DoCommand(ctx context.Context,
	cmd map[string]any,
) (map[string]any, error) {
	if svc.DoCommandFunc == nil {
		return svc.Service.DoCommand(ctx, cmd)
	}
	return svc.DoCommandFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (svc *DataManagerService) Close(ctx context.Context) error {
	if svc.CloseFunc == nil {
		if svc.Service == nil {
			return nil
		}
		return svc.Service.Close(ctx)
	}
	return svc.CloseFunc(ctx)
}

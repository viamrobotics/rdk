package inject

import (
	"context"

	// modelpb "go.viam.com/api/proto/viam/model/v1".
	"go.viam.com/rdk/services/datamanager"
)

// DataManagerService represents a fake instance of an data manager
// service.
type DataManagerService struct {
	datamanager.Service
	SyncFunc func(
		ctx context.Context,
	) error
	// DeployFunc func(
	// 	ctx context.Context, req *modelpb.DeployRequest,
	// ) (*modelpb.DeployResponse, error)
}

// Sync calls the injected Sync or the real variant.
func (svc *DataManagerService) Sync(
	ctx context.Context,
) error {
	if svc.SyncFunc == nil {
		return svc.Service.Sync(ctx)
	}
	return svc.SyncFunc(ctx)
}

// type ModelManagerService struct {
// 	datamanager.MService
// 	DeployFunc func(
// 		ctx context.Context, req *modelpb.DeployRequest,
// 	) (*modelpb.DeployResponse, error)
// }

// func (svc *ModelManagerService) Deploy(
// 	ctx context.Context, req *modelpb.DeployRequest,
// ) (*modelpb.DeployResponse, error) {
// 	if svc.DeployFunc == nil {
// 		return svc.MService.Deploy(ctx, req)
// 	}
// 	return svc.DeployFunc(ctx, req)
// }

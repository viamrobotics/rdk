package inject

import (
	"context"

	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	GetPositionFunc      func(ctx context.Context, name string) (spatialmath.Pose, string, error)
	GetPointCloudMapFunc func(ctx context.Context, name string) (func() ([]byte, error), error)
	GetInternalStateFunc func(ctx context.Context, name string) (func() ([]byte, error), error)
	DoCommandFunc        func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// GetPosition calls the injected GetPositionFunc or the real version.
func (slamSvc *SLAMService) GetPosition(ctx context.Context, name string) (spatialmath.Pose, string, error) {
	if slamSvc.GetPositionFunc == nil {
		return slamSvc.Service.GetPosition(ctx, name)
	}
	return slamSvc.GetPositionFunc(ctx, name)
}

// GetPointCloudMap calls the injected GetPointCloudMap or the real version.
func (slamSvc *SLAMService) GetPointCloudMap(ctx context.Context, name string) (func() ([]byte, error), error) {
	if slamSvc.GetPointCloudMapFunc == nil {
		return slamSvc.Service.GetPointCloudMap(ctx, name)
	}
	return slamSvc.GetPointCloudMapFunc(ctx, name)
}

// GetInternalState calls the injected GetInternalState or the real version.
func (slamSvc *SLAMService) GetInternalState(ctx context.Context, name string) (func() ([]byte, error), error) {
	if slamSvc.GetInternalStateFunc == nil {
		return slamSvc.Service.GetInternalState(ctx, name)
	}
	return slamSvc.GetInternalStateFunc(ctx, name)
}

// DoCommand calls the injected DoCommand or the real variant.
func (slamSvc *SLAMService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if slamSvc.DoCommandFunc == nil {
		return slamSvc.Service.DoCommand(ctx, cmd)
	}
	return slamSvc.DoCommandFunc(ctx, cmd)
}

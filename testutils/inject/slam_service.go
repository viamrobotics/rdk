package inject

import (
	"context"

	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	GetPositionFunc      func(ctx context.Context) (spatialmath.Pose, string, error)
	GetPointCloudMapFunc func(ctx context.Context) (func() ([]byte, error), error)
	GetInternalStateFunc func(ctx context.Context) (func() ([]byte, error), error)
	DoCommandFunc        func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// GetPosition calls the injected GetPositionFunc or the real version.
func (slamSvc *SLAMService) GetPosition(ctx context.Context) (spatialmath.Pose, string, error) {
	if slamSvc.GetPositionFunc == nil {
		return slamSvc.Service.GetPosition(ctx)
	}
	return slamSvc.GetPositionFunc(ctx)
}

// GetPointCloudMap calls the injected GetPointCloudMap or the real version.
func (slamSvc *SLAMService) GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	if slamSvc.GetPointCloudMapFunc == nil {
		return slamSvc.Service.GetPointCloudMap(ctx)
	}
	return slamSvc.GetPointCloudMapFunc(ctx)
}

// GetInternalState calls the injected GetInternalState or the real version.
func (slamSvc *SLAMService) GetInternalState(ctx context.Context) (func() ([]byte, error), error) {
	if slamSvc.GetInternalStateFunc == nil {
		return slamSvc.Service.GetInternalState(ctx)
	}
	return slamSvc.GetInternalStateFunc(ctx)
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

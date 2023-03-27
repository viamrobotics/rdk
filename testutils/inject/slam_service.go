package inject

import (
	"context"

	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	GetPositionFunc            func(ctx context.Context, name string) (spatialmath.Pose, string, error)
	GetPointCloudMapStreamFunc func(ctx context.Context, name string) (func() ([]byte, error), error)
	GetInternalStateStreamFunc func(ctx context.Context, name string) (func() ([]byte, error), error)
	DoCommandFunc              func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// GetPosition calls the injected GetPositionFunc or the real version.
func (slamSvc *SLAMService) GetPosition(ctx context.Context, name string) (spatialmath.Pose, string, error) {
	if slamSvc.GetPositionFunc == nil {
		return slamSvc.Service.GetPosition(ctx, name)
	}
	return slamSvc.GetPositionFunc(ctx, name)
}

// GetPointCloudMapStream calls the injected GetPointCloudMapStream or the real version.
func (slamSvc *SLAMService) GetPointCloudMapStream(ctx context.Context, name string) (func() ([]byte, error), error) {
	if slamSvc.GetPointCloudMapStreamFunc == nil {
		return slamSvc.Service.GetPointCloudMapStream(ctx, name)
	}
	return slamSvc.GetPointCloudMapStreamFunc(ctx, name)
}

// GetInternalStateStream calls the injected GetInternalStateStream or the real version.
func (slamSvc *SLAMService) GetInternalStateStream(ctx context.Context, name string) (func() ([]byte, error), error) {
	if slamSvc.GetInternalStateStreamFunc == nil {
		return slamSvc.Service.GetInternalStateStream(ctx, name)
	}
	return slamSvc.GetInternalStateStreamFunc(ctx, name)
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

package inject

import (
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/services/slam"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	GetPositionFunc func(ctx context.Context, name string) (*commonpb.PoseInFrame, error)
	GetMapFunc      func(ctx context.Context, name string, mimeType string, cp *commonpb.Pose,
		include bool) (string, []byte, *commonpb.PointCloudObject, error)
	CloseFunc func() error
}

// Close calls the injected CloseFunc or the real version.
func (slamSvc *SLAMService) Close() error {
	if slamSvc.CloseFunc == nil {
		return slamSvc.Service.Close()
	}
	return slamSvc.CloseFunc()
}

// GetPosition calls the injected GetPositionFunc or the real version.
func (slamSvc *SLAMService) GetPosition(ctx context.Context, name string) (*commonpb.PoseInFrame, error) {
	if slamSvc.GetPositionFunc == nil {
		return slamSvc.Service.GetPosition(ctx, name)
	}
	return slamSvc.GetPositionFunc(ctx, name)
}

// GetMap calls the injected GetMapFunc or the real version.
func (slamSvc *SLAMService) GetMap(ctx context.Context, name, mimeType string, cp *commonpb.Pose, include bool) (
	string, []byte, *commonpb.PointCloudObject, error) {
	if slamSvc.GetMapFunc == nil {
		return slamSvc.Service.GetMap(ctx, name, mimeType, cp, include)
	}
	return slamSvc.GetMapFunc(ctx, name, mimeType, cp, include)
}

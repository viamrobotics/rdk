package inject

import (
	"context"
	"image"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/vision"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	GetPositionFunc func(ctx context.Context, name string) (*referenceframe.PoseInFrame, error)
	GetMapFunc      func(ctx context.Context, name string, mimeType string, cp *referenceframe.PoseInFrame,
		include bool) (string, image.Image, *vision.Object, error)
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
func (slamSvc *SLAMService) GetPosition(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
	if slamSvc.GetPositionFunc == nil {
		return slamSvc.Service.GetPosition(ctx, name)
	}
	return slamSvc.GetPositionFunc(ctx, name)
}

// GetMap calls the injected GetMapFunc or the real version.
func (slamSvc *SLAMService) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame, include bool) (
	string, image.Image, *vision.Object, error,
) {
	if slamSvc.GetMapFunc == nil {
		return slamSvc.Service.GetMap(ctx, name, mimeType, cp, include)
	}
	return slamSvc.GetMapFunc(ctx, name, mimeType, cp, include)
}

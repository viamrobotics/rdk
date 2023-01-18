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
	PositionFunc func(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error)
	GetMapFunc   func(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
		include bool, extra map[string]interface{}) (string, image.Image, *vision.Object, error)
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Position calls the injected PositionFunc or the real version.
func (slamSvc *SLAMService) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	if slamSvc.PositionFunc == nil {
		return slamSvc.Service.Position(ctx, name, extra)
	}
	return slamSvc.PositionFunc(ctx, name, extra)
}

// GetMap calls the injected GetMapFunc or the real version.
func (slamSvc *SLAMService) GetMap(
	ctx context.Context,
	name, mimeType string,
	cp *referenceframe.PoseInFrame,
	include bool,
	extra map[string]interface{},
) (
	string, image.Image, *vision.Object, error,
) {
	if slamSvc.GetMapFunc == nil {
		return slamSvc.Service.GetMap(ctx, name, mimeType, cp, include, extra)
	}
	return slamSvc.GetMapFunc(ctx, name, mimeType, cp, include, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (slamSvc *SLAMService) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if slamSvc.DoFunc == nil {
		return slamSvc.Service.DoCommand(ctx, cmd)
	}
	return slamSvc.DoFunc(ctx, cmd)
}

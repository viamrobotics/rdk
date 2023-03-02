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
	GetInternalStateFunc       func(ctx context.Context, name string) ([]byte, error)
	GetPointCloudMapStreamFunc func(ctx context.Context, name string) (func() ([]byte, error), error)
	GetInternalStateStreamFunc func(ctx context.Context, name string) (func() ([]byte, error), error)
	DoCommandFunc              func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
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

// GetInternalState calls the injected GetInternalStateFunc or the real version.
func (slamSvc *SLAMService) GetInternalState(ctx context.Context, name string) ([]byte, error) {
	if slamSvc.GetInternalStateFunc == nil {
		return slamSvc.Service.GetInternalState(ctx, name)
	}
	return slamSvc.GetInternalStateFunc(ctx, name)
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
	if slamSvc.GetInternalStateFunc == nil {
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
